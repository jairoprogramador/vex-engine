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

// Un step saltado por falta de comandos no persiste estado (spec 04 §5.3).
//
// El efecto dañino de tratarlo como `success` no era el vocabulario: era que el
// step guardaba el almacén sobre cero comandos ejecutados, así que quedaba
// escrito «sin cambios» para siempre.
func TestStepExecutable_StepSaltadoNoPersisteEstado(t *testing.T) {
	t.Run("control: un step exitoso SÍ guarda lo que produjo", func(t *testing.T) {
		almacen := &varsStoreSpy{}
		ejecutable := domStep.NewStepExecutable(
			handlerQueProduceVariable{}, almacen, &statusRepositorySpy{})

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.NotZero(t, almacen.guardados)
	})

	t.Run("un step saltado no guarda nada", func(t *testing.T) {
		almacen := &varsStoreSpy{}
		status := &statusRepositorySpy{err: errDelBorrado}
		ejecutable := domStep.NewStepExecutable(handlerQueSalta{}, almacen, status)

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.Zero(t, almacen.guardados,
			"las variables declaradas estaban en el mapa acumulado, pero ningún comando las consumió")
		require.Zero(t, status.llamadas,
			"un skip no es un fallo: no hay estado marcado que revertir")
	})
}

// La limpieza del step corre aunque el step falle (spec 06 §5.1).
//
// `step_workdir` lo pone el `before` del step y lo retira su `after`, que hasta
// la spec 06 vivía detrás del camino feliz. El residuo no es cosmético: el mapa
// acumulado es el material de identidad del que salen la huella de variables
// —y mañana `cache_key` y `content_id`— así que dejar ahí el workdir del step
// fallido es dejar residuo en la identidad.
func TestStepExecutable_UnStepFallidoNoDejaSuWorkdirEnElMapaAcumulado(t *testing.T) {
	t.Run("control: el step exitoso tampoco lo deja", func(t *testing.T) {
		contexto := contextoDePrueba(t)
		ejecutable := domStep.NewStepExecutable(
			handlerQueFalla{}, &varsStoreSpy{}, &statusRepositorySpy{})

		require.NoError(t, ejecutable.Execute(contexto))

		_, presente := contexto.GetAccumulatedVar(command.VarStepWorkdir)
		require.False(t, presente)
	})

	t.Run("el step fallido tampoco", func(t *testing.T) {
		contexto := contextoDePrueba(t)
		ejecutable := domStep.NewStepExecutable(
			handlerQueFalla{err: errDelHandler}, &varsStoreSpy{}, &statusRepositorySpy{})

		require.ErrorIs(t, ejecutable.Execute(contexto), errDelHandler)

		_, presente := contexto.GetAccumulatedVar(command.VarStepWorkdir)
		require.False(t, presente,
			"el workdir del step que falló seguiría contaminando la identidad del siguiente")
	})
}

// ── Dobles ──────────────────────────────────────────────────────────────────

type handlerQueFalla struct{ err error }

func (h handlerQueFalla) Handle(_ *context.Context, _ *domStep.StepRequestHandler) error {
	return h.err
}

func (h handlerQueFalla) SetNext(domStep.StepHandler) {}

// handlerQueProduceVariable deja una variable en el mapa acumulado, que es lo que
// el almacén persiste al terminar el step.
type handlerQueProduceVariable struct{}

func (handlerQueProduceVariable) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	variable, err := command.NewVariable("acr_name", "vexsand-demo-app", false)
	if err != nil {
		return err
	}
	request.AddAccumulatedVars(variable)
	return nil
}

func (handlerQueProduceVariable) SetNext(domStep.StepHandler) {}

// handlerQueSalta reproduce lo que hace el handler 04 con un commands.yaml vacío:
// las variables declaradas ya están en el mapa, pero ningún comando corrió.
type handlerQueSalta struct{}

func (handlerQueSalta) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	variable, err := command.NewVariable("acr_name", "vexsand-demo-app", false)
	if err != nil {
		return err
	}
	request.AddAccumulatedVars(variable)
	request.MarkStepSkipped(domStep.SkipReasonNoCommands)
	return nil
}

func (handlerQueSalta) SetNext(domStep.StepHandler) {}

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
