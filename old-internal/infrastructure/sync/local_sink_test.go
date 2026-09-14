package sync_test

// Las propiedades del adaptador de archivo que §7 pide comprobar «por conteo de
// eventos y por hash del directorio destino». Se comprueban aquí y no en el
// harness porque son propiedades del TRANSPORTE —reenviar no duplica, perder el
// puntero no duplica— y observarlas de punta a punta obligaría a poder llamar al
// empuje desde fuera del motor, que es justo lo que no se quiere exponer.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domDeployment "github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	domRecord "github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
	domSync "github.com/jairoprogramador/vex-engine/old-internal/domain/sync"
	recordInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/record"
	syncInfra "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/sync"
)

// ── Fixture ─────────────────────────────────────────────────────────────────

func tiraDePrueba(t *testing.T) domRecord.EventStream {
	t.Helper()

	id, err := domDeployment.ParseDeploymentID(
		domDeployment.DeploymentIDVersion + ":" + strings.Repeat("ab", 32))
	require.NoError(t, err)
	return domRecord.EventStream{Deployment: id, ExecutionID: "exec-1"}
}

func contenidoDePrueba(t *testing.T) domDeployment.Content {
	t.Helper()

	huella := func(version, digito string) fingerprint.Fingerprint {
		f, err := fingerprint.Parse(version + ":" + strings.Repeat(digito, 32))
		require.NoError(t, err)
		return f
	}

	subject, err := domDeployment.NewSubject("https://vex.test/acme/demo-app")
	require.NoError(t, err)
	operation, err := domDeployment.NewOperation("deploy")
	require.NoError(t, err)
	destination, err := domDeployment.NewDestination("sand")
	require.NoError(t, err)
	source, err := domDeployment.NewSource(
		huella(fingerprint.Version, "33"), huella(fingerprint.Version, "44"))
	require.NoError(t, err)
	format, err := domDeployment.NewFormat(2, true)
	require.NoError(t, err)

	stepContent, err := domDeployment.NewStepContent(
		"01-test", domStep.NoStepConfig(), huella(fingerprint.DeclarationVersion, "11"), nil)
	require.NoError(t, err)

	content, err := domDeployment.NewContent(
		subject, operation, destination, source, format,
		[]domDeployment.StepContent{stepContent})
	require.NoError(t, err)
	return content
}

func mustSeq(t *testing.T, position uint64) domRecord.Seq {
	t.Helper()
	seq, err := domRecord.NewSeq(position)
	require.NoError(t, err)
	return seq
}

type escenario struct {
	t         *testing.T
	staging   string
	destino   string
	sink      domSync.Sink
	tira      domRecord.EventStream
	contenido domDeployment.Content
}

func nuevoEscenario(t *testing.T) *escenario {
	t.Helper()

	base := t.TempDir()
	e := &escenario{
		t:         t,
		staging:   filepath.Join(base, "staging"),
		destino:   filepath.Join(base, "destino"),
		tira:      tiraDePrueba(t),
		contenido: contenidoDePrueba(t),
	}
	require.NoError(t, os.MkdirAll(e.staging, 0o755))
	require.NoError(t, os.MkdirAll(e.destino, 0o755))
	e.sink = syncInfra.NewLocalSink(e.staging, e.destino)
	return e
}

// escribirTira materializa la tira del área de trabajo con las posiciones dadas.
// Lo que el empuje transporta es la LÍNEA tal cual, así que basta con un sobre
// creíble.
func (e *escenario) escribirTira(posiciones ...uint64) {
	e.t.Helper()

	path := filepath.Join(e.staging, recordInfra.EventsDirName, recordInfra.StreamRelPath(e.tira))
	require.NoError(e.t, os.MkdirAll(filepath.Dir(path), 0o755))

	var buffer strings.Builder
	for _, seq := range posiciones {
		fmt.Fprintf(&buffer,
			`{"schema_version":1,"event_id":"ev-%d","seq":%d,"at":"2026-08-10T10:00:00Z",`+
				`"attempt":1,"type":"step_started","payload":{"step_id":"01-test"}}`+"\n", seq, seq)
	}
	require.NoError(e.t, os.WriteFile(path, []byte(buffer.String()), 0o644))
}

func (e *escenario) empujar(desde uint64) (domRecord.Seq, error) {
	e.t.Helper()

	from := domRecord.Seq{}
	if desde > 0 {
		from = mustSeq(e.t, desde)
	}
	batch, err := domSync.NewBatch(e.tira, from, e.contenido, domDeployment.ObjectMetadata{
		ProjectCommit: "abc123", PipelineCommit: "def456",
	})
	require.NoError(e.t, err)

	ctx := context.Background()
	return e.sink.Push(&ctx, batch)
}

// posicionesDelDestino son los `seq` que el destino tiene, en el orden del
// archivo.
func (e *escenario) posicionesDelDestino() []uint64 {
	e.t.Helper()

	path := filepath.Join(e.destino, recordInfra.EventsDirName, recordInfra.StreamRelPath(e.tira))
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(e.t, err)

	posiciones := make([]uint64, 0, 8)
	for _, linea := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(linea) == "" {
			continue
		}
		var sobre struct {
			Seq uint64 `json:"seq"`
		}
		require.NoError(e.t, json.Unmarshal([]byte(linea), &sobre))
		posiciones = append(posiciones, sobre.Seq)
	}
	return posiciones
}

// huellaDelDestino es el hash del árbol entero: rutas y contenidos. Es la forma
// literal de «empujar dos veces deja el destino idéntico».
func (e *escenario) huellaDelDestino() string {
	e.t.Helper()

	rutas := make([]string, 0, 8)
	require.NoError(e.t, filepath.WalkDir(e.destino, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rutas = append(rutas, path)
		return nil
	}))
	sort.Strings(rutas)

	suma := sha256.New()
	for _, path := range rutas {
		rel, err := filepath.Rel(e.destino, path)
		require.NoError(e.t, err)
		data, err := os.ReadFile(path)
		require.NoError(e.t, err)
		suma.Write([]byte(rel))
		suma.Write(data)
	}
	return hex.EncodeToString(suma.Sum(nil))
}

// ── Casos ───────────────────────────────────────────────────────────────────

func TestLocalSink_EmpujaElObjetoYLosHechos(t *testing.T) {
	e := nuevoEscenario(t)
	e.escribirTira(1, 2, 3)

	confirmado, err := e.empujar(0)
	require.NoError(t, err)

	assert.Equal(t, uint64(3), confirmado.Position())
	assert.Equal(t, []uint64{1, 2, 3}, e.posicionesDelDestino())

	objetos, err := filepath.Glob(filepath.Join(
		e.destino, "objects", e.contenido.ID().Version(), "*", "*.json"))
	require.NoError(t, err)
	assert.Len(t, objetos, 1, "la intención llegó, direccionada por su contenido")
}

func TestLocalSink_EmpujarElMismoLoteDosVecesDejaElDestinoIdentico(t *testing.T) {
	e := nuevoEscenario(t)
	e.escribirTira(1, 2, 3)

	_, err := e.empujar(0)
	require.NoError(t, err)
	antes := e.huellaDelDestino()

	confirmado, err := e.empujar(0)
	require.NoError(t, err)

	assert.Equal(t, antes, e.huellaDelDestino(),
		"reenviar lo mismo no produce nada: ni una línea, ni un objeto reescrito")
	assert.Equal(t, []uint64{1, 2, 3}, e.posicionesDelDestino())
	assert.Equal(t, uint64(3), confirmado.Position())
}

func TestLocalSink_ReempujarDesdeCeroContraUnDestinoPobladoNoDuplica(t *testing.T) {
	// Es la propiedad que convierte al `ack` en una optimización: forzar la
	// posición cero —que es lo que pasa cuando su archivo desaparece— produce el
	// mismo destino.
	e := nuevoEscenario(t)
	e.escribirTira(1, 2)
	_, err := e.empujar(0)
	require.NoError(t, err)

	e.escribirTira(1, 2, 3, 4)
	confirmado, err := e.empujar(0)
	require.NoError(t, err)

	assert.Equal(t, []uint64{1, 2, 3, 4}, e.posicionesDelDestino(),
		"sólo se anexa lo que el destino no tenía, aunque el lote diga «desde el principio»")
	assert.Equal(t, uint64(4), confirmado.Position())
}

func TestLocalSink_ElAckPorDelanteDelDestinoNoSaltaHechos(t *testing.T) {
	// El suelo es el MAYOR de los dos, así que un `ack` adelantado no puede dejar
	// un hueco en el destino... pero tampoco puede retroceder el destino. Aquí lo
	// que se comprueba es que el destino manda cuando va por delante.
	e := nuevoEscenario(t)
	e.escribirTira(1, 2, 3)
	_, err := e.empujar(0)
	require.NoError(t, err)

	confirmado, err := e.empujar(1)
	require.NoError(t, err)

	assert.Equal(t, []uint64{1, 2, 3}, e.posicionesDelDestino())
	assert.Equal(t, uint64(3), confirmado.Position())
}

func TestLocalSink_UnDestinoInalcanzableFallaRuidosamente(t *testing.T) {
	// `local` NO es un no-op: es el test que la spec 16 anunció y que aquí se
	// ejecuta de verdad. Sin él, un volumen no montado dejaría al motor
	// «sincronizando» contra el filesystem efímero del contenedor.
	e := nuevoEscenario(t)
	e.escribirTira(1)
	require.NoError(t, os.RemoveAll(e.destino))

	_, err := e.empujar(0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), e.destino)
	assert.Contains(t, err.Error(), "volumen")
}

func TestLocalSink_UnaTiraQueNoEstaEnElAreaDeTrabajoEsUnaAnomalia(t *testing.T) {
	// Devolver «no había nada pendiente» se leería como un empuje que funcionó, y
	// el objeto ya habría llegado. Es preferible el error con la ruta dentro.
	e := nuevoEscenario(t)

	_, err := e.empujar(0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "área de trabajo")
}

func TestLocalSink_UnaLineaAMediasEnElDestinoNoSePisaYLoNuevoSigueSiendoLegible(t *testing.T) {
	// Lo que una muerte dura deja: media línea al final del archivo. Del destino
	// no se borra nada, así que lo que se hace es cerrarla y seguir anexando.
	e := nuevoEscenario(t)
	e.escribirTira(1, 2)
	_, err := e.empujar(0)
	require.NoError(t, err)

	destino := filepath.Join(e.destino, recordInfra.EventsDirName, recordInfra.StreamRelPath(e.tira))
	roto, err := os.OpenFile(destino, os.O_APPEND|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	_, err = roto.WriteString(`{"schema_version":1,"seq":3,"ty`)
	require.NoError(t, err)
	require.NoError(t, roto.Close())

	e.escribirTira(1, 2, 3, 4)
	_, err = e.empujar(0)
	require.NoError(t, err)

	data, err := os.ReadFile(destino)
	require.NoError(t, err)
	lineas := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	require.Len(t, lineas, 5, "las dos buenas, la rota entera, y las dos nuevas")
	assert.Equal(t, `{"schema_version":1,"seq":3,"ty`, lineas[2],
		"la línea rota se conserva tal cual: del destino no se borra nada")

	var ultima struct {
		Seq uint64 `json:"seq"`
	}
	require.NoError(t, json.Unmarshal([]byte(lineas[4]), &ultima))
	assert.Equal(t, uint64(4), ultima.Seq)
}
