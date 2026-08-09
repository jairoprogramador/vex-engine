package record_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
)

// unaEjecucionCompleta es la tira de hechos de un `deploy` que va bien: dos
// steps, uno ejecutado y otro revivido.
func unaEjecucionCompleta(t *testing.T) []record.Event {
	t.Helper()

	return []record.Event{
		evento(t, 1, 0, record.AttemptStarted{
			Deployment: despliegueDePrueba(t), Actor: "jairo", Runner: "fly-machine"}),
		evento(t, 2, 1, record.StepStarted{
			StepID: "01-test", Scope: ambienteDePrueba(t), StepFingerprint: "ck-v1:aaa"}),
		evento(t, 3, 2, record.CommandStarted{StepID: "01-test", CommandName: "mvn test"}),
		evento(t, 4, 3, record.CommandFinished{
			StepID: "01-test", CommandName: "mvn test",
			Status: command.CommandSuccess, Duration: time.Second}),
		evento(t, 5, 4, record.StepFinished{
			StepID: "01-test", Scope: ambienteDePrueba(t),
			Status: command.StepSuccess, Duration: 3 * time.Second,
			Evidence: evidenciaDePrueba(t)}),
		evento(t, 6, 5, record.StepStarted{
			StepID: "02-supply", Scope: ambienteDePrueba(t), StepFingerprint: "ck-v1:bbb"}),
		evento(t, 7, 6, record.StepFinished{
			StepID: "02-supply", Scope: ambienteDePrueba(t),
			Status: command.StepCached, FromCache: true, Evidence: evidenciaDePrueba(t)}),
		evento(t, 8, 7, record.AttemptFinished{Status: record.AttemptSucceeded}),
	}
}

func TestFold_UnaEjecucionCompleta(t *testing.T) {
	resultado := record.Fold(unaEjecucionCompleta(t))

	assert.Equal(t, record.AttemptSucceeded, resultado.Status)
	assert.True(t, resultado.IsTerminal())
	assert.True(t, resultado.Deployment.Equals(despliegueDePrueba(t)))
	assert.Equal(t, 1, resultado.Attempt.Number())
	assert.Equal(t, "jairo", resultado.Actor)
	assert.Equal(t, "fly-machine", resultado.Runner)
	assert.Equal(t, origen, resultado.StartedAt)
	assert.Equal(t, origen.Add(7*time.Second), resultado.FinishedAt)
	assert.Equal(t, 7*time.Second, resultado.Duration)
	assert.Equal(t, "02-supply", resultado.LastStep)
	assert.Equal(t, 1, resultado.Commands)
	assert.Zero(t, resultado.FailedCommands)
	assert.Nil(t, resultado.ExitCode)
	assert.True(t, resultado.ErrorClass.IsZero())

	require.Len(t, resultado.Steps, 2)
	assert.Equal(t, "01-test", resultado.Steps[0].StepID)
	assert.Equal(t, command.StepSuccess, resultado.Steps[0].Status)
	assert.True(t, resultado.Steps[0].Finished)
	assert.Equal(t, "ck-v1:aaa", resultado.Steps[0].StepFingerprint)
	assert.False(t, resultado.Steps[0].FromCache)

	_, hay := resultado.Step("99-inexistente")
	assert.False(t, hay)

	revivido, hay := resultado.Step("02-supply")
	require.True(t, hay)
	assert.True(t, revivido.FromCache)
	assert.Equal(t, command.StepCached, revivido.Status)
	assert.Equal(t, evidenciaDePrueba(t), revivido.Evidence,
		"un step revivido dice de qué registro se fía: es la respuesta a "+
			"«¿cuándo se testeó esto por última vez?»")
}

// ── El test que justifica el modelo entero ─────────────────────────────────

func TestFold_SinAttemptFinishedElResultadoIGUALEXISTE(t *testing.T) {
	completa := unaEjecucionCompleta(t)
	// Se corta la tira donde la cortaría una Fly Machine que muere: a mitad del
	// segundo step, con su `step_started` escrito y su cierre nunca.
	interrumpida := completa[:6]

	resultado := record.Fold(interrumpida)

	assert.Equal(t, record.AttemptInterrupted, resultado.Status)
	assert.False(t, resultado.IsTerminal())
	assert.Equal(t, "02-supply", resultado.LastStep,
		"dice exactamente hasta dónde se llegó")
	assert.True(t, resultado.FinishedAt.IsZero())
	assert.Zero(t, resultado.Duration, "un intento interrumpido tiene principio, no duración")

	abierto, hay := resultado.Step("02-supply")
	require.True(t, hay)
	assert.False(t, abierto.Finished, "empezó y no terminó, que no es lo mismo que fallar")
	assert.Equal(t, command.StepRunning, abierto.Status)
}

func TestFold_UnSliceVacioPliegaAInterrumpido(t *testing.T) {
	resultado := record.Fold(nil)

	assert.Equal(t, record.AttemptInterrupted, resultado.Status)
	assert.Empty(t, resultado.Steps)
	assert.Empty(t, resultado.LastStep)
	assert.True(t, resultado.Attempt.IsZero())
	assert.True(t, resultado.Deployment.IsZero())
}

// ── Pureza ─────────────────────────────────────────────────────────────────

func TestFold_EsPura(t *testing.T) {
	hechos := unaEjecucionCompleta(t)

	uno := record.Fold(hechos)
	otro := record.Fold(hechos)

	assert.Equal(t, uno, otro, "mismo slice, mismo resultado")
}

func TestFold_OrdenaPorSeqNuncaPorTime(t *testing.T) {
	hechos := unaEjecucionCompleta(t)
	esperado := record.Fold(hechos)

	// Se desordena la tira: el reloj no es confiable —contenedor, máquina remota,
	// ajuste NTP— y el orden sí importa para plegar.
	desordenados := []record.Event{
		hechos[7], hechos[2], hechos[0], hechos[6],
		hechos[4], hechos[1], hechos[5], hechos[3],
	}

	assert.Equal(t, esperado, record.Fold(desordenados))
}

func TestFold_NoMutaElSliceQueRecibe(t *testing.T) {
	hechos := unaEjecucionCompleta(t)
	desordenados := []record.Event{hechos[7], hechos[0], hechos[1]}
	copia := append([]record.Event(nil), desordenados...)

	record.Fold(desordenados)

	assert.Equal(t, copia, desordenados,
		"un plegado que reordena los hechos de quien lo llama no es puro por mucho que lo parezca")
}

// ── Fallo, cancelación y conteos ───────────────────────────────────────────

func TestFold_ElFalloDelIntentoEsElDelPRIMERStepQueFallo(t *testing.T) {
	hechos := []record.Event{
		evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t)}),
		evento(t, 2, 1, record.StepStarted{StepID: "01-test"}),
		evento(t, 3, 2, record.CommandFinished{
			StepID: "01-test", CommandName: "mvn test",
			Status: command.CommandFailure, ExitCode: 2}),
		evento(t, 4, 3, record.StepFinished{
			StepID: "01-test", Status: command.StepFailure,
			ExitCode: codigo(2), ErrorClass: record.ErrorClassCommandFailed}),
		evento(t, 5, 4, record.StepStarted{StepID: "02-supply"}),
		evento(t, 6, 5, record.StepFinished{
			StepID: "02-supply", Status: command.StepFailure,
			ExitCode: codigo(9), ErrorClass: record.ErrorClassUnknown}),
		evento(t, 7, 6, record.AttemptFinished{Status: record.AttemptFailed}),
	}

	resultado := record.Fold(hechos)

	assert.Equal(t, record.AttemptFailed, resultado.Status)
	require.NotNil(t, resultado.ExitCode)
	assert.Equal(t, 2, *resultado.ExitCode)
	assert.Equal(t, record.ErrorClassCommandFailed, resultado.ErrorClass)
	assert.Equal(t, 1, resultado.Commands)
	assert.Equal(t, 1, resultado.FailedCommands)
}

func TestFold_LaCancelacionSeDistingueDeLaInterrupcion(t *testing.T) {
	hechos := []record.Event{
		evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t)}),
		evento(t, 2, 1, record.StepStarted{StepID: "01-test"}),
		evento(t, 3, 2, record.AttemptFinished{Status: record.AttemptCancelled}),
	}

	resultado := record.Fold(hechos)

	assert.Equal(t, record.AttemptCancelled, resultado.Status)
	assert.True(t, resultado.IsTerminal(),
		"una cancelación se ESCRIBE; una interrupción se deduce de que nadie escribió")
}

func TestFold_CuentaArtefactosYClonesViejos(t *testing.T) {
	hechos := []record.Event{
		evento(t, 1, 0, record.AttemptStarted{Deployment: despliegueDePrueba(t)}),
		evento(t, 2, 1, record.StaleCloneUsed{
			Source: "https://vex.test/acme/pipelinecode", AgeHours: 5}),
		evento(t, 3, 2, record.ArtifactProduced{
			StepID: "03-package", Kind: "image", Digest: "sha256:abc"}),
		evento(t, 4, 3, record.ParameterResolved{
			Name: "image", Source: command.OriginResolved, Digest: "sha256:abc"}),
		evento(t, 5, 4, record.SyncFailed{Destination: "supabase", Cause: "timeout"}),
		evento(t, 6, 5, record.AttemptFinished{Status: record.AttemptSucceeded}),
	}

	resultado := record.Fold(hechos)

	assert.Equal(t, 1, resultado.Artifacts)
	assert.Equal(t, 1, resultado.StaleClones)
	assert.Equal(t, record.AttemptSucceeded, resultado.Status)
}

// ── Bordes de una tira incompleta o ajena ──────────────────────────────────

func TestFold_UnCierreSinAperturaNoSeDescarta(t *testing.T) {
	// Un archivo truncado POR DELANTE es exactamente cómo se ve una máquina
	// efímera que murió y dejó la cola. El hecho ocurrió: perderlo no ayuda.
	hechos := []record.Event{
		evento(t, 5, 0, record.StepFinished{
			StepID: "03-deploy", Status: command.StepSuccess, Duration: time.Second}),
	}

	resultado := record.Fold(hechos)

	require.Len(t, resultado.Steps, 1)
	assert.Equal(t, "03-deploy", resultado.Steps[0].StepID)
	assert.True(t, resultado.Steps[0].Finished)
	assert.Equal(t, 1, resultado.Attempt.Number(),
		"el intento se toma del sobre del primer hecho, aunque falte attempt_started")
	assert.Equal(t, record.AttemptInterrupted, resultado.Status)
}

func TestFold_UnMismoStepAbiertoDosVecesNoSeDuplica(t *testing.T) {
	hechos := []record.Event{
		evento(t, 1, 0, record.StepStarted{StepID: "01-test"}),
		evento(t, 2, 1, record.StepStarted{StepID: "01-test"}),
		evento(t, 3, 2, record.StepFinished{StepID: "01-test", Status: command.StepSuccess}),
	}

	resultado := record.Fold(hechos)

	require.Len(t, resultado.Steps, 1)
	assert.True(t, resultado.Steps[0].Finished)
}

func TestFold_IgnoraLoQueNoEntiende(t *testing.T) {
	// La mitad LECTORA del OCP: un hecho escrito por un motor más nuevo no
	// invalida el pliegue de los que sí se entienden.
	hechos := append(unaEjecucionCompleta(t), eventoDeTipoDesconocido(t))

	resultado := record.Fold(hechos)

	assert.Equal(t, record.AttemptSucceeded, resultado.Status)
	assert.Len(t, resultado.Steps, 2)
}

func TestFold_IgnoraElValorCero(t *testing.T) {
	hechos := append([]record.Event{{}}, unaEjecucionCompleta(t)...)

	assert.Equal(t, record.Fold(unaEjecucionCompleta(t)), record.Fold(hechos))
}

// cargaDesconocida simula un tipo de hecho que este motor no conoce.
type cargaDesconocida struct{}

func (cargaDesconocida) Type() record.EventType { return record.EventType("lo_que_venga") }
func (cargaDesconocida) Validate() error        { return nil }

func eventoDeTipoDesconocido(t *testing.T) record.Event {
	t.Helper()
	return evento(t, 99, 99, cargaDesconocida{})
}
