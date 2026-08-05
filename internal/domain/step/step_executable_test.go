package step_test

// El borrado de estado tras un step fallido es COMPENSATORIO: revierte lo que
// el step alcanzó a marcar antes de fallar. Descartar su error dejaba el step
// marcado como exitoso habiendo fallado, y la siguiente ejecución lo saltaba
// (spec 02 §5.4).

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

var (
	errDelHandler = errors.New("el comando salió con 1")
	errDelBorrado = errors.New("permiso denegado al borrar el estado")
)

func TestStepExecutable_ElFalloDelBorradoCompensatorioSePropaga(t *testing.T) {
	status := &statusRepositorySpy{err: errDelBorrado}
	ejecutable := domStep.NewStepExecutable(
		handlerQueFalla{err: errDelHandler},
		&varsStoreSpy{},
		status,
	)

	err := ejecutable.Execute(contextoDePrueba(t))

	require.ErrorIs(t, err, errDelHandler, "el error original del step no se puede perder")
	require.ErrorIs(t, err, errDelBorrado, "el fallo al revertir el estado tiene que hacer fallar el step")
	require.Equal(t, 1, status.llamadas)
}

func TestStepExecutable_StepFallidoConBorradoCorrectoSoloReportaElFalloDelStep(t *testing.T) {
	status := &statusRepositorySpy{}
	ejecutable := domStep.NewStepExecutable(
		handlerQueFalla{err: errDelHandler},
		&varsStoreSpy{},
		status,
	)

	err := ejecutable.Execute(contextoDePrueba(t))

	require.ErrorIs(t, err, errDelHandler)
	require.Equal(t, 1, status.llamadas)
}

func TestStepExecutable_StepExitosoNoBorraElEstado(t *testing.T) {
	status := &statusRepositorySpy{err: errDelBorrado}
	almacen := &varsStoreSpy{}
	ejecutable := domStep.NewStepExecutable(handlerQueFalla{}, almacen, status)

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
	require.Zero(t, status.llamadas)
}

// ── Dobles ──────────────────────────────────────────────────────────────────

type handlerQueFalla struct{ err error }

func (h handlerQueFalla) Handle(_ *context.Context, _ *domStep.StepRequestHandler) error {
	return h.err
}

func (h handlerQueFalla) SetNext(domStep.StepHandler) {}

type statusRepositorySpy struct {
	err      error
	llamadas int
}

var _ domStepStatus.StatusRepository = (*statusRepositorySpy)(nil)

func (s *statusRepositorySpy) Delete(_, _, _, _ string) error {
	s.llamadas++
	return s.err
}

type varsStoreSpy struct{ guardados int }

var _ domStep.VarsStoreRepository = (*varsStoreSpy)(nil)

func (v *varsStoreSpy) Get(*context.Context, string, string, string, string) ([]command.Variable, error) {
	return []command.Variable{}, nil
}

func (v *varsStoreSpy) Save(_ *context.Context, _, _, _, _ string, _ []command.Variable) error {
	v.guardados++
	return nil
}

// ── Fixture ─────────────────────────────────────────────────────────────────

type emisorMudo struct{}

func (emisorMudo) Notify(string, string) {}

func contextoDePrueba(t *testing.T) *command.ExecutionContext {
	t.Helper()

	ctx := context.Background()
	ejecucion := command.NewExecution(
		command.NewExecutionID("exec-1"),
		command.NewExecutionProject("id", "proyecto", "https://vex.test/org/proyecto.git", "main", "org", "equipo"),
		command.NewExecutionPipeline("https://vex.test/org/pipeline.git", "main"),
		"supply",
		"prod",
		command.NewExecutionRuntime("", ""),
	)

	executionContext := command.NewExecutionContext(&ctx, ejecucion, nil, nil, emisorMudo{}, nil)

	stepName, err := command.NewStepName("02-supply")
	require.NoError(t, err)
	executionContext.SetStepName(stepName)
	executionContext.SetWorkdir(t.TempDir())

	return executionContext
}
