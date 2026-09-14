package command_test

// El ciclo de vida del agregado (spec 07).
//
// `Execution` tenía transiciones y ningún llamador: el status se quedaba en
// `queued` de principio a fin, `finishedAt` y `exitCode` eran siempre nil, y
// quien decidía si la ejecución había ido bien era la CLI, tres capas afuera,
// mirando un `error`. Lo que estos casos fijan es que el agregado sea la
// autoridad sobre su propio estado.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/shared"
)

var (
	instanteInicio = time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC)
	instanteFin    = time.Date(2026, 3, 4, 10, 5, 0, 0, time.UTC)
)

// relojQueAvanza devuelve el primer instante en la primera llamada y el
// segundo en las siguientes: es lo mínimo para que startedAt y finishedAt no
// coincidan sin traer el reloj de pared a la prueba.
type relojQueAvanza struct {
	instantes []time.Time
	llamadas  int
}

func (r *relojQueAvanza) Now() time.Time {
	instante := r.instantes[min(r.llamadas, len(r.instantes)-1)]
	r.llamadas++
	return instante
}

func nuevaEjecucion(clock shared.Clock) *command.Execution {
	return command.NewExecution(
		command.NewExecutionID("exec-1"),
		command.NewExecutionProject("id", "proyecto", "https://vex.test/org/proyecto.git", "main", "org", "equipo"),
		command.NewExecutionPipeline("https://vex.test/org/pipeline.git", "main"),
		"supply",
		"prod",
		command.NewExecutionRuntime("", ""),
		clock,
	)
}

func TestExecution_UnaEjecucionExitosaSeCierraConSusInstantes(t *testing.T) {
	ejecucion := nuevaEjecucion(&relojQueAvanza{instantes: []time.Time{instanteInicio, instanteFin}})

	require.Equal(t, command.StatusQueued, ejecucion.Status())
	require.Nil(t, ejecucion.FinishedAt())
	require.Nil(t, ejecucion.ExitCode())

	require.NoError(t, ejecucion.MarkRunning())
	assert.Equal(t, command.StatusRunning, ejecucion.Status())

	require.NoError(t, ejecucion.MarkSucceeded(0))

	assert.Equal(t, command.StatusSucceeded, ejecucion.Status())
	require.NotNil(t, ejecucion.FinishedAt(), "sin finishedAt no hay duración de nada, en ningún nivel")
	assert.Equal(t, instanteFin, *ejecucion.FinishedAt())
	require.NotNil(t, ejecucion.ExitCode())
	assert.Equal(t, 0, *ejecucion.ExitCode())

	duracion, ok := ejecucion.Duration()
	require.True(t, ok)
	assert.Equal(t, 5*time.Minute, duracion, "el instante viene del reloj inyectado, no de time.Now()")
}

func TestExecution_UnFalloConservaElExitCodeDelComando(t *testing.T) {
	ejecucion := nuevaEjecucion(shared.NewFixedClock(instanteInicio))
	require.NoError(t, ejecucion.MarkRunning())

	require.NoError(t, ejecucion.MarkFailed(3))

	assert.Equal(t, command.StatusFailed, ejecucion.Status())
	require.NotNil(t, ejecucion.ExitCode())
	assert.Equal(t, 3, *ejecucion.ExitCode())
}

func TestExecution_UnaCancelacionNoLlevaExitCode(t *testing.T) {
	ejecucion := nuevaEjecucion(shared.NewFixedClock(instanteInicio))
	require.NoError(t, ejecucion.MarkRunning())

	require.NoError(t, ejecucion.MarkCancelled())

	assert.Equal(t, command.StatusCancelled, ejecucion.Status())
	assert.NotNil(t, ejecucion.FinishedAt())
	assert.Nil(t, ejecucion.ExitCode(),
		"no hay comando cuyo resultado reportar: es la diferencia con un fallo")
}

// Una señal puede llegar antes de que la cadena arranque. Cancelar algo que
// aún no empezó es legítimo; lo que no es legítimo es darlo por terminado.
func TestExecution_SeCancelaTambienAntesDeEmpezar(t *testing.T) {
	ejecucion := nuevaEjecucion(shared.NewFixedClock(instanteInicio))

	require.NoError(t, ejecucion.MarkCancelled())
	assert.Equal(t, command.StatusCancelled, ejecucion.Status())
}

func TestExecution_TransicionesIlegales(t *testing.T) {
	casos := []struct {
		nombre         string
		preparar       func(*command.Execution)
		transicion     func(*command.Execution) error
		estadoEsperado command.ExecutionStatus
	}{
		{
			nombre:         "éxito después de fallo",
			preparar:       func(e *command.Execution) { _ = e.MarkRunning(); _ = e.MarkFailed(3) },
			transicion:     func(e *command.Execution) error { return e.MarkSucceeded(0) },
			estadoEsperado: command.StatusFailed,
		},
		{
			// El caso que la cancelación necesita: cancelar mata el comando en
			// curso, la cadena devuelve error y el use case intenta marcar el
			// fallo. Ese fallo es CONSECUENCIA de la cancelación, no un hecho
			// nuevo, y no debe pisarla.
			nombre:         "fallo después de cancelación",
			preparar:       func(e *command.Execution) { _ = e.MarkRunning(); _ = e.MarkCancelled() },
			transicion:     func(e *command.Execution) error { return e.MarkFailed(1) },
			estadoEsperado: command.StatusCancelled,
		},
		{
			nombre:         "terminar sin haber empezado",
			preparar:       func(*command.Execution) {},
			transicion:     func(e *command.Execution) error { return e.MarkSucceeded(0) },
			estadoEsperado: command.StatusQueued,
		},
		{
			nombre:         "empezar dos veces",
			preparar:       func(e *command.Execution) { _ = e.MarkRunning() },
			transicion:     func(e *command.Execution) error { return e.MarkRunning() },
			estadoEsperado: command.StatusRunning,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			ejecucion := nuevaEjecucion(shared.NewFixedClock(instanteInicio))
			caso.preparar(ejecucion)

			err := caso.transicion(ejecucion)

			require.ErrorIs(t, err, command.ErrTransicionIlegal)
			assert.Equal(t, caso.estadoEsperado, ejecucion.Status(),
				"una transición rechazada no corrompe el estado en silencio")
		})
	}
}

// Los dos hechos que colgaban de PipelineRequestHandler —el estado de una
// cadena— y que son de la ejecución (spec 07 §5.3).
func TestExecution_VersionYCommitSonHechosDeLaEjecucion(t *testing.T) {
	ejecucion := nuevaEjecucion(shared.NewFixedClock(instanteInicio))

	ejecucion.SetProjectVersion("v2601011200")
	ejecucion.SetProjectHeadHash("abc123def456")

	assert.Equal(t, "v2601011200", ejecucion.ProjectVersion())
	assert.Equal(t, "abc123def456", ejecucion.ProjectHeadHash())
}
