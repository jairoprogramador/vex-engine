package record_test

// La forma SERIALIZADA de los diez hechos.
//
// Es lo que un ingestor de otra plataforma va a leer, así que se fija aquí y no
// sólo de refilón en el harness: un campo que cambia de nombre o de tipo rompe a
// un consumidor que este repositorio no ve.
//
// La spec 18 dejó ocho tipos sin traducción a propósito —fijar el formato de un
// hecho sin ver el dato que lo llena es cómo se congelan campos que luego no
// encajan— y su `default` devolvía un error nombrando a la spec 19. Aquí entran
// los ocho.

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	domRecord "github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
	infraRecord "github.com/jairoprogramador/vex-engine/internal/infrastructure/record"
)

var instante = time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)

// TODOS los tipos del vocabulario tienen forma serializada. El `default` del
// traductor es un error en ejecución y el compilador no ayuda, así que este caso
// es lo que avisa de un tipo añadido y no traducido.
func TestToJSONLEventDTO_TodoElVocabularioSeSerializa(t *testing.T) {
	for tipo, carga := range cargasDeCadaTipo(t) {
		t.Run(tipo.String(), func(t *testing.T) {
			dto, err := infraRecord.ToJSONLEventDTO(evento(t, carga))
			require.NoError(t, err, "el hecho '%s' no tiene forma serializada", tipo)

			assert.Equal(t, tipo.String(), dto.Type)
			assert.Equal(t, 1, dto.SchemaVersion)
			assert.Equal(t, uint64(1), dto.Seq)
			assert.Equal(t, 1, dto.Attempt)
			assert.NotEmpty(t, dto.EventID)
			assert.NotEmpty(t, dto.At)
			assert.NotEmpty(t, dto.Payload, "una línea muda es peor que una ausente")
		})
	}
}

// El hecho con más carga del vocabulario, entero.
func TestToJSONLEventDTO_StepFinished(t *testing.T) {
	codigo := 7
	dto, err := infraRecord.ToJSONLEventDTO(evento(t, domRecord.StepFinished{
		StepID:          "02-supply",
		Scope:           ambiente(t),
		Status:          command.StepFailure,
		Duration:        1500 * time.Millisecond,
		Reason:          command.ReasonChanged,
		StepFingerprint: "ck-v1:" + strings.Repeat("ab", 32),
		Evidence:        evidencia(t),
		ExitCode:        &codigo,
		ErrorClass:      domRecord.ErrorClassCommandFailed,
	}))
	require.NoError(t, err)

	assert.Equal(t, "02-supply", dto.Payload["step_id"])
	assert.Equal(t, "environment:sand", dto.Payload["scope"],
		"el ámbito viaja como CAMPO, no dentro de un hash: se puede filtrar sin descomponer nada")
	assert.Equal(t, "FAILURE", dto.Payload["status"])
	assert.Equal(t, "changed", dto.Payload["reason"])
	assert.Equal(t, false, dto.Payload["from_cache"])
	assert.Equal(t, 7, dto.Payload["exit_code"])
	assert.Equal(t, "command_failed", dto.Payload["error_class"])

	// La unidad es de la SERIALIZACIÓN, no del dominio: dentro es `time.Duration`
	// y aquí `duration_ms`, en entero para que el consumidor pueda agregar.
	assert.Equal(t, int64(1500), dto.Payload["duration_ms"])

	// La clave de estado por COMPONENTES, igual que el índice: `Key.String()` es
	// una forma legible para diagnósticos y lo dice de sí misma, no un formato de
	// serialización — componerla obligaría a inventar un escape del separador.
	clave, ok := dto.Payload["evidence_from"].(map[string]any)
	require.True(t, ok)
	stateKey, ok := clave["state_key"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "https://vex.test/acme/demo-app", stateKey["subject"])
	assert.Equal(t, "environment:sand", stateKey["scope"])
	assert.Equal(t, "02-supply", stateKey["step_id"])
	assert.NotEmpty(t, clave["record_id"])

	// `deployment_id` y `attempt` NO están, y su ausencia es una verdad:
	// `state.Provenance` no sabe todavía de qué despliegue salió el registro.
	assert.NotContains(t, clave, "deployment_id")
	assert.NotContains(t, clave, "attempt")
}

// LOS CAMPOS OPCIONALES SE OMITEN, no se escriben en cero. La ausencia de una
// clave es «no consta»; su presencia es un hecho. Un `exit_code: 0` en un step
// que revivió afirmaría que un proceso salió con éxito, y no hubo ninguno.
func TestToJSONLEventDTO_LoQueNoConstaNoSeEscribe(t *testing.T) {
	dto, err := infraRecord.ToJSONLEventDTO(evento(t, domRecord.StepFinished{
		StepID:    "01-test",
		Scope:     ambiente(t),
		Status:    command.StepCached,
		FromCache: true,
		Reason:    command.ReasonUpToDate,
	}))
	require.NoError(t, err)

	assert.Equal(t, true, dto.Payload["from_cache"])
	assert.NotContains(t, dto.Payload, "exit_code",
		"un step revivido no tiene código de salida que reportar")
	assert.NotContains(t, dto.Payload, "error_class")
	assert.NotContains(t, dto.Payload, "evidence_from")
	assert.NotContains(t, dto.Payload, "step_fingerprint",
		"la huella vacía es legítima —un step que no declara `state_changed`— y no se escribe")
}

// A diferencia del de un step, el exit code de un COMANDO va siempre y como
// valor: todo comando que termina tiene uno, y el cero significa lo que
// significa.
func TestToJSONLEventDTO_ElExitCodeDeUnComandoVaSiempre(t *testing.T) {
	dto, err := infraRecord.ToJSONLEventDTO(evento(t, domRecord.CommandFinished{
		StepID:      "01-test",
		CommandName: "build",
		Status:      command.CommandSuccess,
		Duration:    250 * time.Millisecond,
	}))
	require.NoError(t, err)

	assert.Equal(t, 0, dto.Payload["exit_code"])
	assert.NotContains(t, dto.Payload, "error_class")
}

// ── Fixture ─────────────────────────────────────────────────────────────────

func cargasDeCadaTipo(t *testing.T) map[domRecord.EventType]domRecord.Payload {
	t.Helper()

	return map[domRecord.EventType]domRecord.Payload{
		domRecord.TypeAttemptStarted: domRecord.AttemptStarted{
			Deployment: despliegue(t), Runner: "vex-runtime:1"},
		domRecord.TypeStaleCloneUsed: domRecord.StaleCloneUsed{
			Source: "https://vex.test/acme/pipelinecode", AgeHours: 3},
		domRecord.TypeStepStarted: domRecord.StepStarted{StepID: "01-test"},
		domRecord.TypeStepFinished: domRecord.StepFinished{
			StepID: "01-test", Status: command.StepSuccess, Reason: command.ReasonNoRecord},
		domRecord.TypeCommandStarted: domRecord.CommandStarted{
			StepID: "01-test", CommandName: "build"},
		domRecord.TypeCommandFinished: domRecord.CommandFinished{
			StepID: "01-test", CommandName: "build", Status: command.CommandSuccess},
		domRecord.TypeParameterResolved: domRecord.ParameterResolved{
			Name: "acr_name", Source: command.OriginRuntime, Digest: domRecord.DigestOf("vexsand")},
		domRecord.TypeArtifactProduced: domRecord.ArtifactProduced{
			StepID: "03-package", Kind: "image", Digest: "sha256:abc"},
		domRecord.TypeSyncFailed: domRecord.SyncFailed{
			Destination: "https://portal.vex.test", Cause: "502"},
		domRecord.TypeAttemptFinished: domRecord.AttemptFinished{
			Status: domRecord.AttemptSucceeded},
	}
}

func evento(t *testing.T, carga domRecord.Payload) domRecord.Event {
	t.Helper()

	id, err := domRecord.NewEventID(instante, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)

	hecho, err := domRecord.NewEvent(
		id, domRecord.FirstSeq(), instante, deployment.FirstAttempt(), carga)
	require.NoError(t, err)
	return hecho
}

func despliegue(t *testing.T) deployment.DeploymentID {
	t.Helper()
	id, err := deployment.ParseDeploymentID(
		deployment.DeploymentIDVersion + ":" + strings.Repeat("cd", 32))
	require.NoError(t, err)
	return id
}

func ambiente(t *testing.T) state.Scope {
	t.Helper()
	scope, err := state.NewEnvironmentScope("sand")
	require.NoError(t, err)
	return scope
}

func evidencia(t *testing.T) domRecord.EvidenceRef {
	t.Helper()

	key, err := state.NewKey("https://vex.test/acme/demo-app", ambiente(t), "02-supply")
	require.NoError(t, err)
	recordID, err := state.NewRecordID(instante.Add(-24*time.Hour),
		[]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)

	return domRecord.EvidenceRef{
		ExecutionID: "e5a1f0c2-0000-4000-8000-000000000001",
		At:          instante.Add(-24 * time.Hour),
		StateKey:    key,
		RecordID:    recordID,
	}
}
