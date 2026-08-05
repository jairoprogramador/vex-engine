package step_test

// El estado de re-ejecución se escribe UNA sola vez, desde aquí, y solo si el
// step terminó bien (spec 09 §5.2).
//
// Aquí vivía el borrado compensatorio: las reglas escribían la huella nueva
// dentro de `Evaluate` —antes del primer comando— y este `Delete` la revertía si
// el step fallaba. La spec 02 §5.4 hizo que su error dejara de descartarse; la
// spec 09 lo elimina junto con su causa, porque un compensador solo revierte lo
// que el programa alcanza a ejecutar, y la muerte dura —la que de verdad dejaba
// un `deploy` saltado— nunca pasa por él.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

var (
	errDelHandler  = errors.New("el comando salió con 1")
	errDeEscritura = errors.New("permiso denegado al escribir el estado")
)

func TestStepExecutable_ElEstadoSeEscribeSoloTrasElExito(t *testing.T) {
	t.Run("un step exitoso escribe lo que la policy observó", func(t *testing.T) {
		almacen := &statusSpy{}
		ejecutable := domStep.NewStepExecutable(
			handlerQueEvalua{}, &varsStoreSpy{}, almacen.writer())

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

		require.Equal(t, []string{"huella-inst"}, almacen.inst.escritas)
	})

	// Una muerte dura es, vista desde el código, «se evaluó la policy y no se
	// llegó al camino de éxito». Antes la huella ya estaba escrita para entonces
	// —la escribía `Evaluate`— y solo el borrado compensatorio la quitaba, que es
	// código que corre después y que un SIGKILL no ejecuta.
	//
	// Lo que este test fija es la propiedad ESTRUCTURAL que sustituye a aquel
	// compensador: por este camino no se escribe nada, así que no hay nada que
	// revertir. El test que sí cambia de color con la spec 09 es el de
	// idempotencia de `Evaluate` (rules_test.go), donde la escritura anticipada
	// era directamente observable.
	t.Run("un step que evalúa y NO termina no deja estado escrito", func(t *testing.T) {
		almacen := &statusSpy{}
		ejecutable := domStep.NewStepExecutable(
			handlerQueEvaluaYFalla{err: errDelHandler}, &varsStoreSpy{}, almacen.writer())

		require.ErrorIs(t, ejecutable.Execute(contextoDePrueba(t)), errDelHandler)
		require.Empty(t, almacen.inst.escritas,
			"la evidencia anotada muere con la cadena; nada llegó al disco")
	})

	t.Run("un step saltado no escribe estado", func(t *testing.T) {
		almacen := &statusSpy{}
		ejecutable := domStep.NewStepExecutable(
			handlerQueSalta{}, &varsStoreSpy{}, almacen.writer())

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.Empty(t, almacen.inst.escritas)
	})
}

// La escritura ocurre DESPUÉS del último comando, no antes ni durante. Es la
// ventana de la spec 09 §1(a) medida por orden, no supuesta.
func TestStepExecutable_ElEstadoSeEscribeDespuesDelUltimoComando(t *testing.T) {
	almacen := &statusSpy{}
	escritor := almacen.writer()
	ejecutable := domStep.NewStepExecutable(
		handlerQueEvalua{orden: &almacen.orden}, &varsStoreSpy{}, escritor)

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

	require.Equal(t, []string{"comando", "escritura"}, almacen.orden)
}

// (b) de la spec 09 §1: si la escritura falla, el step no falla —el despliegue
// ocurrió— pero el usuario se entera. Antes el fallo del `Set` se convertía en
// una razón para ejecutar dentro de la propia decisión, y era indistinguible de
// «el código cambió».
func TestStepExecutable_UnFalloAlEscribirElEstadoNoTumbaElStepPeroSeDice(t *testing.T) {
	almacen := &statusSpy{}
	almacen.inst.err = errDeEscritura
	emisor := &emisorEspia{}
	contexto := contextoDePruebaCon(t, emisor)

	ejecutable := domStep.NewStepExecutable(
		handlerQueEvalua{}, &varsStoreSpy{}, almacen.writer())

	require.NoError(t, ejecutable.Execute(contexto),
		"no haber podido guardar el caché no invalida el despliegue que sí ocurrió")
	assert.True(t, emisor.contiene("no se pudo guardar el estado de re-ejecución"),
		"pero deja de ser silencioso: el usuario merece saber que su caché está roto")
	assert.False(t, emisor.contiene("cambió"),
		"y no se disfraza de cambio de contenido")
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
			handlerQueProduceVariable{}, almacen, (&statusSpy{}).writer())

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.NotZero(t, almacen.guardados)
	})

	t.Run("un step saltado no guarda nada", func(t *testing.T) {
		almacen := &varsStoreSpy{}
		estado := &statusSpy{}
		ejecutable := domStep.NewStepExecutable(handlerQueSalta{}, almacen, estado.writer())

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.Zero(t, almacen.guardados,
			"las variables declaradas estaban en el mapa acumulado, pero ningún comando las consumió")
		require.Empty(t, estado.inst.escritas,
			"ni el estado de re-ejecución: no se ejecutó nada que justifique escribirlo")
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
			handlerQueFalla{}, &varsStoreSpy{}, (&statusSpy{}).writer())

		require.NoError(t, ejecutable.Execute(contexto))

		_, presente := contexto.GetAccumulatedVar(command.VarStepWorkdir)
		require.False(t, presente)
	})

	t.Run("el step fallido tampoco", func(t *testing.T) {
		contexto := contextoDePrueba(t)
		ejecutable := domStep.NewStepExecutable(
			handlerQueFalla{err: errDelHandler}, &varsStoreSpy{}, (&statusSpy{}).writer())

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

// handlerQueEvalua reproduce lo que hace el handler 04 cuando la policy manda
// ejecutar: anota la evidencia y corre los comandos. Anotar no persiste nada
// —eso lo hace el camino de éxito de StepExecutable—, que es justo lo que estos
// tests miden.
type handlerQueEvalua struct{ orden *[]string }

func (h handlerQueEvalua) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	request.RecordStatusEvidence(contextoDeReglas(request), evidenciaDePrueba())
	if h.orden != nil {
		*h.orden = append(*h.orden, "comando")
	}
	return nil
}

func (handlerQueEvalua) SetNext(domStep.StepHandler) {}

// handlerQueEvaluaYFalla es la muerte a mitad: la policy ya se evaluó, los
// comandos empezaron y el step no llegó al final.
type handlerQueEvaluaYFalla struct{ err error }

func (h handlerQueEvaluaYFalla) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	request.RecordStatusEvidence(contextoDeReglas(request), evidenciaDePrueba())
	return h.err
}

func (handlerQueEvaluaYFalla) SetNext(domStep.StepHandler) {}

func contextoDeReglas(request *domStep.StepRequestHandler) domStepStatus.RuleContext {
	return domStepStatus.RuleContext{
		domStepStatus.ProjectUrlParam:  request.ProjectUrl(),
		domStepStatus.PipelineUrlParam: request.PipelineUrl(),
		domStepStatus.EnvironmentParam: request.Environment(),
		domStepStatus.StepParam:        request.StepName(),
	}
}

func evidenciaDePrueba() []domStepStatus.Evidence {
	return []domStepStatus.Evidence{
		domStepStatus.NewEvidence(domStepStatus.InstPipelineRuleName, "huella-inst", "", true),
	}
}

// ── Espía del almacén de estado ─────────────────────────────────────────────
//
// Cuenta escrituras y su orden respecto de los comandos. La spec 09 §7 pide
// exactamente eso: cero `Set` al consultar, y uno por regla después de que el
// último comando terminó.

type statusSpy struct {
	inst  instRepoEspia
	orden []string
}

func (s *statusSpy) writer() *domStepStatus.StatusWriter {
	s.inst.orden = &s.orden
	return domStepStatus.NewStatusWriter(&s.inst, varsRepoMudo{}, codeRepoMudo{}, timeRepoMudo{})
}

type instRepoEspia struct {
	escritas []string
	err      error
	orden    *[]string
}

var _ domStepStatus.InstructionsStatusRepository = (*instRepoEspia)(nil)

func (r *instRepoEspia) Get(_, _, _ string) (string, error) { return "", nil }

func (r *instRepoEspia) Set(_, _, _, fingerprint string) error {
	if r.orden != nil {
		*r.orden = append(*r.orden, "escritura")
	}
	if r.err != nil {
		return r.err
	}
	r.escritas = append(r.escritas, fingerprint)
	return nil
}

func (r *instRepoEspia) Delete(_, _, _ string) error { return nil }

type varsRepoMudo struct{}

var _ domStepStatus.VariablesStatusRepository = varsRepoMudo{}

func (varsRepoMudo) Get(_, _, _, _ string) (string, error) { return "", nil }
func (varsRepoMudo) Set(_, _, _, _, _ string) error        { return nil }
func (varsRepoMudo) Delete(_, _, _, _ string) error        { return nil }

type codeRepoMudo struct{}

var _ domStepStatus.CodeStatusRepository = codeRepoMudo{}

func (codeRepoMudo) Get(_, _, _ string) (string, error) { return "", nil }
func (codeRepoMudo) Set(_, _, _, _ string) error        { return nil }
func (codeRepoMudo) Delete(_, _, _ string) error        { return nil }

type timeRepoMudo struct{}

var _ domStepStatus.TimeStatusRepository = timeRepoMudo{}

func (timeRepoMudo) Get(_, _, _ string) (time.Time, error) { return time.Time{}, nil }
func (timeRepoMudo) Set(_, _, _ string, _ time.Time) error { return nil }
func (timeRepoMudo) Delete(_, _, _ string) error           { return nil }

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

// emisorEspia guarda las líneas para poder afirmar qué se le dijo al usuario.
type emisorEspia struct{ lineas []string }

func (e *emisorEspia) Notify(_, line string) { e.lineas = append(e.lineas, line) }

func (e *emisorEspia) contiene(fragmento string) bool {
	for _, linea := range e.lineas {
		if strings.Contains(linea, fragmento) {
			return true
		}
	}
	return false
}

func contextoDePrueba(t *testing.T) *command.ExecutionContext {
	t.Helper()
	return contextoDePruebaCon(t, emisorMudo{})
}

func contextoDePruebaCon(t *testing.T, emisor domNotify.LogObserver) *command.ExecutionContext {
	t.Helper()

	ctx := context.Background()
	ejecucion := command.NewExecution(
		command.NewExecutionID("exec-1"),
		command.NewExecutionProject("id", "proyecto", "https://vex.test/org/proyecto.git", "main", "org", "equipo"),
		command.NewExecutionPipeline("https://vex.test/org/pipeline.git", "main"),
		"supply",
		"prod",
		command.NewExecutionRuntime("", ""),
		shared.NewFixedClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
	)

	executionContext := command.NewExecutionContext(&ctx, ejecucion, nil, nil, emisor, nil)

	stepName, err := command.NewStepName("02-supply")
	require.NoError(t, err)
	executionContext.SetStepName(stepName)
	executionContext.SetWorkdir(t.TempDir())

	return executionContext
}
