package record_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

func TestEvent_ElSobreLlevaIdentidadPosicionInstanteEIntento(t *testing.T) {
	hecho := evento(t, 1, 0, record.AttemptStarted{
		Deployment: despliegueDePrueba(t),
		Actor:      "jairo",
		Runner:     "fly-machine",
	})

	assert.False(t, hecho.ID().IsZero())
	assert.Equal(t, uint64(1), hecho.Seq().Position())
	assert.Equal(t, origen, hecho.At())
	assert.Equal(t, 1, hecho.Attempt().Number())
	assert.Equal(t, record.TypeAttemptStarted, hecho.Type())
	assert.False(t, hecho.IsZero())
	assert.NotNil(t, hecho.Payload())
}

func TestEvent_UnSobreIncompletoEsUnError(t *testing.T) {
	id, err := record.NewEventID(origen, entropiaFija)
	require.NoError(t, err)
	carga := record.AttemptFinished{Status: record.AttemptSucceeded}

	casos := map[string]func() (record.Event, error){
		"sin event_id": func() (record.Event, error) {
			return record.NewEvent(
				record.EventID{}, record.FirstSeq(), origen, deployment.FirstAttempt(), carga)
		},
		"sin seq": func() (record.Event, error) {
			return record.NewEvent(
				id, record.Seq{}, origen, deployment.FirstAttempt(), carga)
		},
		"sin instante": func() (record.Event, error) {
			return record.NewEvent(
				id, record.FirstSeq(), time.Time{}, deployment.FirstAttempt(), carga)
		},
		"sin intento": func() (record.Event, error) {
			return record.NewEvent(
				id, record.FirstSeq(), origen, deployment.Attempt{}, carga)
		},
		"sin carga": func() (record.Event, error) {
			return record.NewEvent(
				id, record.FirstSeq(), origen, deployment.FirstAttempt(), nil)
		},
		"con una carga inválida": func() (record.Event, error) {
			return record.NewEvent(
				id, record.FirstSeq(), origen, deployment.FirstAttempt(),
				record.StepStarted{})
		},
	}

	for nombre, componer := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := componer()
			require.Error(t, err)
		})
	}
}

func TestEvent_ElValorCeroNoEsUnHecho(t *testing.T) {
	var hecho record.Event
	assert.True(t, hecho.IsZero())
	assert.Empty(t, hecho.Type().String())
}

func TestPayloads_CadaTipoDiceElSuyo(t *testing.T) {
	casos := map[record.EventType]record.Payload{
		record.TypeAttemptStarted:    record.AttemptStarted{},
		record.TypeStepStarted:       record.StepStarted{},
		record.TypeStepFinished:      record.StepFinished{},
		record.TypeCommandStarted:    record.CommandStarted{},
		record.TypeCommandFinished:   record.CommandFinished{},
		record.TypeParameterResolved: record.ParameterResolved{},
		record.TypeArtifactProduced:  record.ArtifactProduced{},
		record.TypeStaleCloneUsed:    record.StaleCloneUsed{},
		record.TypeSyncFailed:        record.SyncFailed{},
		record.TypeAttemptFinished:   record.AttemptFinished{},
	}

	for esperado, carga := range casos {
		assert.Equal(t, esperado, carga.Type())
	}
}

func TestPayloads_Invariantes(t *testing.T) {
	validas := map[string]record.Payload{
		"attempt_started": record.AttemptStarted{Deployment: despliegueDePrueba(t)},
		"step_started con huella": record.StepStarted{
			StepID: "01-test", Scope: ambienteDePrueba(t), StepFingerprint: "ck-v1:abc"},
		"step_started SIN huella, que es legítimo desde la spec 15": record.StepStarted{
			StepID: "01-test"},
		"step_finished con éxito": record.StepFinished{
			StepID: "01-test", Status: command.StepSuccess},
		"step_finished revivido con evidencia": record.StepFinished{
			StepID: "02-supply", Status: command.StepCached,
			FromCache: true, Evidence: evidenciaDePrueba(t)},
		"step_finished saltado por no tener comandos": record.StepFinished{
			StepID: "05-notify", Status: command.StepSkipped},
		"step_finished fallido": record.StepFinished{
			StepID: "03-deploy", Status: command.StepFailure,
			ExitCode: codigo(2), ErrorClass: record.ErrorClassCommandFailed},
		"command_started":  record.CommandStarted{StepID: "01-test", CommandName: "mvn test"},
		"command_finished": record.CommandFinished{StepID: "01-test", CommandName: "mvn test", Status: command.CommandSuccess},
		"parameter_resolved": record.ParameterResolved{
			Name: "image", Source: command.OriginResolved, Digest: "sha256:abc"},
		"artifact_produced": record.ArtifactProduced{
			StepID: "03-package", Kind: "image", Digest: "sha256:abc"},
		"stale_clone_used": record.StaleCloneUsed{
			Source: "https://vex.test/acme/pipelinecode", AgeHours: 3.5},
		"sync_failed":      record.SyncFailed{Destination: "supabase", Cause: "timeout"},
		"attempt_finished": record.AttemptFinished{Status: record.AttemptFailed},
	}
	for nombre, carga := range validas {
		t.Run(nombre, func(t *testing.T) {
			require.NoError(t, carga.Validate())
		})
	}

	invalidas := map[string]record.Payload{
		"attempt_started sin deployment_id": record.AttemptStarted{},
		"step_started sin step_id":          record.StepStarted{},
		"step_finished sin step_id":         record.StepFinished{Status: command.StepSuccess},
		"step_finished con un estado no terminal": record.StepFinished{
			StepID: "01-test", Status: command.StepRunning},
		"step_finished con duración negativa": record.StepFinished{
			StepID: "01-test", Status: command.StepSuccess, Duration: -time.Second},
		"command_started sin step_id": record.CommandStarted{CommandName: "mvn"},
		"command_started sin nombre":  record.CommandStarted{StepID: "01-test"},
		"command_finished sin nombre": record.CommandFinished{StepID: "01-test", Status: command.CommandSuccess},
		"command_finished con un estado que no es de comando": record.CommandFinished{
			StepID: "01-test", CommandName: "mvn", Status: command.CommandStatus("LO_QUE_SEA")},
		"command_finished con duración negativa": record.CommandFinished{
			StepID: "01-test", CommandName: "mvn",
			Status: command.CommandSuccess, Duration: -time.Second},
		"parameter_resolved sin nombre": record.ParameterResolved{Digest: "sha256:abc"},
		"artifact_produced sin step_id": record.ArtifactProduced{Kind: "image", Digest: "x"},
		"artifact_produced sin tipo":    record.ArtifactProduced{StepID: "03-package", Digest: "x"},
		"artifact_produced sin digest":  record.ArtifactProduced{StepID: "03-package", Kind: "image"},
		"stale_clone_used sin fuente":   record.StaleCloneUsed{AgeHours: 1},
		"stale_clone_used con edad negativa": record.StaleCloneUsed{
			Source: "https://vex.test/acme/pipelinecode", AgeHours: -1},
		"sync_failed sin destino":        record.SyncFailed{Cause: "timeout"},
		"sync_failed sin causa":          record.SyncFailed{Destination: "supabase"},
		"attempt_finished sin desenlace": record.AttemptFinished{},
	}
	for nombre, carga := range invalidas {
		t.Run(nombre, func(t *testing.T) {
			require.Error(t, carga.Validate())
		})
	}
}

func TestAttemptFinished_NoPuedeDeclararseInterrumpido(t *testing.T) {
	// `interrupted` se DERIVA de la ausencia de este hecho: quien está vivo para
	// escribirlo, por definición, no fue interrumpido. Emitirlo sería una
	// contradicción escrita en el registro.
	err := record.AttemptFinished{Status: record.AttemptInterrupted}.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), record.AttemptInterrupted.String())
}

func TestEvidenceRef_UnaEvidenciaAMediasEsPeorQueNinguna(t *testing.T) {
	completa := evidenciaDePrueba(t)
	require.NoError(t, completa.Validate())
	assert.False(t, completa.IsZero())

	// El valor cero es legítimo: los tres casos que ejecutan y no escriben
	// registro —sin `config.yaml`, sin comandos, sin `rules`— no invocan ninguno.
	var ninguna record.EvidenceRef
	assert.True(t, ninguna.IsZero())
	require.NoError(t, ninguna.Validate())

	sinEjecucion := completa
	sinEjecucion.ExecutionID = ""
	require.Error(t, sinEjecucion.Validate())

	sinInstante := completa
	sinInstante.At = time.Time{}
	require.Error(t, sinInstante.Validate())

	sinClave := completa
	sinClave.StateKey = state.Key{}
	require.Error(t, sinClave.Validate())

	sinRegistro := completa
	sinRegistro.RecordID = state.RecordID{}
	require.Error(t, sinRegistro.Validate())
}

func TestEvidenceRef_LosCamposDeDespliegueSonOpcionalesTodavia(t *testing.T) {
	// `state.Provenance` es hoy `{ExecutionID, At}`: un registro escrito antes de
	// que lleve `deployment_id` y `attempt` produce una evidencia sin ellos, y eso
	// es exactamente lo que se sabe de él.
	evidencia := evidenciaDePrueba(t)
	assert.True(t, evidencia.Deployment.IsZero())
	assert.True(t, evidencia.Attempt.IsZero())
	require.NoError(t, evidencia.Validate())

	conDespliegue := evidencia
	conDespliegue.Deployment = despliegueDePrueba(t)
	conDespliegue.Attempt = deployment.FirstAttempt()
	require.NoError(t, conDespliegue.Validate())
}
