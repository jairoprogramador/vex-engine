package record_test

// El lector del JSONL, que la spec 22 PUBLICA.
//
// Hasta aquí vivía dentro de `harness_test.go` y era deliberado: publicar el lector
// antes de tener un consumidor habría fijado una superficie sin nadie detrás
// (spec 19 §9). Estos casos son la ida y vuelta completa —los diez tipos— más las
// dos asimetrías que el lector tiene y el escritor no: un tipo desconocido se salta
// y una línea ilegible se cuenta, esté donde esté.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domRecord "github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	infraRecord "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/record"
)

// LA IDA Y VUELTA DE LOS DIEZ TIPOS. Es lo que hace que emisión y lectura no se
// separen: un campo que un emisor añada y este lector no sepa leer se nota aquí, no
// seis meses después en el consumidor.
func TestFromJSONLEventDTO_TodoElVocabularioVuelve(t *testing.T) {
	for tipo, carga := range cargasDeCadaTipo(t) {
		t.Run(tipo.String(), func(t *testing.T) {
			original := evento(t, carga)
			dto, err := infraRecord.ToJSONLEventDTO(original)
			require.NoError(t, err)

			vuelta, err := infraRecord.FromJSONLEventDTO(dto)
			require.NoError(t, err)

			assert.Equal(t, original.ID(), vuelta.ID())
			assert.Equal(t, original.Seq(), vuelta.Seq())
			assert.True(t, original.At().Equal(vuelta.At()))
			assert.Equal(t, original.Attempt(), vuelta.Attempt())
			assert.Equal(t, original.Type(), vuelta.Type())
			assert.Equal(t, original.Payload(), vuelta.Payload(),
				"la carga vuelve entera o el consumidor pierde un campo en silencio")
		})
	}
}

// EL HECHO CON MÁS CARGA vuelve con sus opcionales, incluida la evidencia
// —que es un mapa anidado y el sitio más fácil de perder algo—.
func TestFromJSONLEventDTO_StepFinishedVuelveConTodo(t *testing.T) {
	codigo := 7
	original := evento(t, domRecord.StepFinished{
		StepID:          "02-supply",
		Scope:           ambiente(t),
		Status:          command.StepFailure,
		Duration:        1500 * time.Millisecond,
		Reason:          command.ReasonChanged,
		StepFingerprint: "ck-v1:" + strings.Repeat("ab", 32),
		Evidence:        evidencia(t),
		ExitCode:        &codigo,
		ErrorClass:      domRecord.ErrorClassCommandFailed,
	})

	dto, err := infraRecord.ToJSONLEventDTO(original)
	require.NoError(t, err)
	vuelta, err := infraRecord.FromJSONLEventDTO(dto)
	require.NoError(t, err)

	carga, ok := vuelta.Payload().(domRecord.StepFinished)
	require.True(t, ok)
	assert.Equal(t, "02-supply", carga.StepID)
	assert.Equal(t, ambiente(t), carga.Scope)
	assert.Equal(t, command.StepFailure, carga.Status)
	assert.Equal(t, 1500*time.Millisecond, carga.Duration)
	assert.Equal(t, command.ReasonChanged, carga.Reason)
	require.NotNil(t, carga.ExitCode)
	assert.Equal(t, 7, *carga.ExitCode)
	assert.Equal(t, domRecord.ErrorClassCommandFailed, carga.ErrorClass)
	assert.Equal(t, evidencia(t), carga.Evidence)
}

// LA AUSENCIA DE UN OPCIONAL VUELVE COMO AUSENCIA, no como cero.
//
// Es la mitad lectora de la disciplina del escritor: `exit_code: 0` en un step que
// revivió afirmaría que un proceso salió con éxito, y no hubo ninguno.
func TestFromJSONLEventDTO_LoOmitidoVuelveComoAusencia(t *testing.T) {
	original := evento(t, domRecord.StepFinished{
		StepID:    "01-test",
		Status:    command.StepCached,
		FromCache: true,
		Reason:    command.ReasonUpToDate,
	})

	dto, err := infraRecord.ToJSONLEventDTO(original)
	require.NoError(t, err)
	vuelta, err := infraRecord.FromJSONLEventDTO(dto)
	require.NoError(t, err)

	carga := vuelta.Payload().(domRecord.StepFinished)
	assert.Nil(t, carga.ExitCode, "un step que revivió no tiene código de salida")
	assert.True(t, carga.ErrorClass.IsZero())
	assert.True(t, carga.Evidence.IsZero())
	assert.True(t, carga.Scope.IsZero())
}

// LA ASIMETRÍA CON EL ESCRITOR, y es la mitad lectora del OCP.
//
// Escribir un tipo sin traducción es un ERROR —el emisor estaría dejando una línea
// que nadie sabe leer—; encontrárselo al leer NO lo es: es un motor más nuevo, y
// negarse a leer la tira por eso perdería los nueve tipos que sí se entienden.
func TestFromJSONLEventDTO_UnTipoDesconocidoNoEsUnArchivoRoto(t *testing.T) {
	dto := infraRecord.JSONLEventDTO{
		SchemaVersion: 1,
		EventID:       "e5a1f0c2-0000-4000-8000-000000000001",
		Seq:           1,
		At:            instante.Format(time.RFC3339Nano),
		Attempt:       1,
		Type:          "rollback_anchored",
		Payload:       map[string]any{"deployment_id": "dep-v1:" + strings.Repeat("cd", 32)},
	}

	_, err := infraRecord.FromJSONLEventDTO(dto)

	require.Error(t, err)
	assert.True(t, errors.Is(err, infraRecord.ErrTipoDesconocido))
	assert.Contains(t, err.Error(), "rollback_anchored",
		"el mensaje nombra el tipo: es lo que dice qué motor lo escribió")
}

// UNA LÍNEA ILEGIBLE **EN MEDIO** se salta, y esto es lo que separa leer el área de
// trabajo de leer el destino.
//
// El empuje nunca borra nada del destino: si una máquina murió a mitad de un
// `write`, la línea rota se CONSERVA y lo nuevo se anexa detrás (spec 21 §9.7). Un
// lector que asumiera «lo ilegible está al final» leería bien el área de trabajo y se
// caería contra el destino, que es justo el que un operador mira cuando algo fue mal.
func TestReadStrip_UnaLineaIlegibleEnMedioNoRompeLaTira(t *testing.T) {
	path := tiraEnDisco(t,
		lineaDe(t, 1, domRecord.AttemptStarted{Deployment: despliegue(t), Runner: "img"}),
		`{"schema_version":1,"seq":2,"at":`, // cortada a mitad de un write
		lineaDe(t, 3, domRecord.AttemptFinished{Status: domRecord.AttemptSucceeded}),
	)

	strip, err := infraRecord.ReadStrip(path)
	require.NoError(t, err)

	assert.Len(t, strip.Events, 2, "los dos hechos legibles entran")
	assert.Equal(t, []int{2}, strip.Ilegibles,
		"y la línea rota se CUENTA: leerla en silencio convertiría una corrupción en un resultado limpio")
	assert.Equal(t, domRecord.AttemptSucceeded, domRecord.Fold(strip.Events).Status)
}

// UNA LÍNEA VACÍA NO ES UN DEFECTO. Es la otra regla de tolerancia que el sink de
// la spec 21 dejó escrita y probada, y aquí se hereda tal cual.
func TestReadStrip_LasLineasVaciasSeSaltanSinContarse(t *testing.T) {
	path := tiraEnDisco(t,
		lineaDe(t, 1, domRecord.AttemptStarted{Deployment: despliegue(t), Runner: "img"}),
		"",
		"   ",
		lineaDe(t, 2, domRecord.AttemptFinished{Status: domRecord.AttemptSucceeded}),
	)

	strip, err := infraRecord.ReadStrip(path)
	require.NoError(t, err)

	assert.Len(t, strip.Events, 2)
	assert.Empty(t, strip.Ilegibles)
}

// UN TIPO DESCONOCIDO SE CUENTA Y SU SOBRE SE CONSERVA.
//
// Los dos a la vez, porque las dos cosas se necesitan: `Fold` no puede plegarlo
// —no hay carga que construir— y `record log` sí tiene que poder imprimirlo. De ahí
// que la tira leída lleve los hechos Y los sobres crudos.
func TestReadStrip_UnTipoDesconocidoSeCuentaYSuSobreSeConserva(t *testing.T) {
	desconocida := infraRecord.JSONLEventDTO{
		SchemaVersion: 1,
		EventID:       "e5a1f0c2-0000-4000-8000-000000000009",
		Seq:           2,
		At:            instante.Format(time.RFC3339Nano),
		Attempt:       1,
		Type:          "rollback_anchored",
		Payload:       map[string]any{"anchor": "algo"},
	}
	crudo, err := json.Marshal(desconocida)
	require.NoError(t, err)

	path := tiraEnDisco(t,
		lineaDe(t, 1, domRecord.AttemptStarted{Deployment: despliegue(t), Runner: "img"}),
		string(crudo),
	)

	strip, err := infraRecord.ReadStrip(path)
	require.NoError(t, err)

	assert.Len(t, strip.Events, 1)
	assert.Equal(t, []string{"rollback_anchored"}, strip.Desconocidos)
	assert.Len(t, strip.Raw, 2, "el sobre se conserva: `record log` lo imprime igual")
	assert.Empty(t, strip.Ilegibles, "un tipo nuevo no es una línea rota")
}

// UN SOBRE QUE EL MODELO RECHAZA cuenta como ilegible, y es la respuesta correcta:
// no se puede plegar, y tumbar la lectura por él perdería el resto.
func TestReadStrip_UnSobreQueElModeloRechazaCuentaComoIlegible(t *testing.T) {
	path := tiraEnDisco(t,
		`{"schema_version":1,"event_id":"","seq":1,"at":"2026-08-09T10:00:00Z","attempt":1,`+
			`"type":"attempt_finished","payload":{"status":"succeeded"}}`,
	)

	strip, err := infraRecord.ReadStrip(path)
	require.NoError(t, err)

	assert.Empty(t, strip.Events)
	assert.Equal(t, []int{1}, strip.Ilegibles)
}

// LA TIRA DICE DE QUÉ EJECUCIÓN ES, y lo dice por el nombre del archivo: es el
// `execution_id`, que es único por construcción y NO es ordenable (decisión I-2).
func TestStrip_LaEjecucionSaleDelNombreDelArchivo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "e5a1f0c2-0000-4000-8000-000000000001.jsonl")
	require.NoError(t, os.WriteFile(path,
		[]byte(lineaDe(t, 1, domRecord.AttemptStarted{
			Deployment: despliegue(t), Runner: "img"})+"\n"), 0o644))

	strip, err := infraRecord.ReadStrip(path)
	require.NoError(t, err)

	assert.Equal(t, "e5a1f0c2-0000-4000-8000-000000000001", strip.ExecutionID())
	assert.Equal(t, 1, strip.Attempt().Number())
}

// ── Utilidades ──────────────────────────────────────────────────────────────

// lineaDe serializa un hecho a su línea, con la posición que se le diga.
func lineaDe(t *testing.T, seq uint64, carga domRecord.Payload) string {
	t.Helper()

	dto, err := infraRecord.ToJSONLEventDTO(evento(t, carga))
	require.NoError(t, err)
	dto.Seq = seq

	crudo, err := json.Marshal(dto)
	require.NoError(t, err)
	return string(crudo)
}

func tiraEnDisco(t *testing.T, lineas ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "e5a1f0c2-0000-4000-8000-000000000001.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lineas, "\n")+"\n"), 0o644))
	return path
}
