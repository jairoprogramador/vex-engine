package step_test

// La entrada de caché se escribe UNA sola vez, desde aquí, y solo si el step
// terminó bien (spec 09 §5.2, heredado por la 10 §5.2).
//
// Aquí vivía el borrado compensatorio: las reglas escribían la huella nueva
// dentro de `Evaluate` —antes del primer comando— y este `Delete` la revertía si
// el step fallaba. La spec 09 lo eliminó junto con su causa; la 10 no cambió el
// MOMENTO —eso ya estaba— sino QUÉ se escribe: donde había un `switch` por
// nombre de regla repartiendo cuatro evidencias a cuatro almacenes, hay una sola
// escritura bajo una sola clave.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domCache "github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

var (
	errDelHandler  = errors.New("el comando salió con 1")
	errDeEscritura = errors.New("permiso denegado al escribir el estado")
)

func TestStepExecutable_LaEntradaSeEscribeSoloTrasElExito(t *testing.T) {
	t.Run("un step exitoso escribe la entrada de su clave", func(t *testing.T) {
		almacen := &entriesEspia{}
		ejecutable := domStep.NewStepExecutable(handlerQueAnota{}, &varsStoreSpy{}, almacen)

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

		require.Equal(t, []string{claveDePrueba(t).String()}, almacen.escritas)
	})

	// Una muerte dura es, vista desde el código, «se decidió ejecutar y no se
	// llegó al camino de éxito». Antes la huella ya estaba escrita para entonces
	// —la escribía `Evaluate`— y solo el borrado compensatorio la quitaba, que es
	// código que corre después y que un SIGKILL no ejecuta.
	//
	// Lo que este test fija es la propiedad ESTRUCTURAL que sustituye a aquel
	// compensador: por este camino no se escribe nada, así que no hay nada que
	// revertir.
	t.Run("un step que anota su clave y NO termina no deja entrada", func(t *testing.T) {
		almacen := &entriesEspia{}
		ejecutable := domStep.NewStepExecutable(
			handlerQueAnotaYFalla{err: errDelHandler}, &varsStoreSpy{}, almacen)

		require.ErrorIs(t, ejecutable.Execute(contextoDePrueba(t)), errDelHandler)
		require.Empty(t, almacen.escritas,
			"la clave anotada muere con la cadena; nada llegó al disco")
	})

	t.Run("un step saltado no escribe entrada", func(t *testing.T) {
		almacen := &entriesEspia{}
		ejecutable := domStep.NewStepExecutable(handlerQueSalta{}, &varsStoreSpy{}, almacen)

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.Empty(t, almacen.escritas)
	})

	// Un step que no anotó clave —porque no se pudo componer su material— no
	// deja entrada aunque termine bien. Escribir bajo una clave incompleta sería
	// peor que no escribir: colisionaría con la de cualquier otro material al
	// que le faltara lo mismo. Y como «ausencia de entrada ⇒ ejecutar», no
	// escribir es exactamente lo correcto (spec 10, §8 última fila del recuadro).
	t.Run("un step sin clave anotada no escribe entrada", func(t *testing.T) {
		almacen := &entriesEspia{}
		ejecutable := domStep.NewStepExecutable(handlerQueNoAnota{}, &varsStoreSpy{}, almacen)

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.Empty(t, almacen.escritas)
	})
}

// La escritura ocurre DESPUÉS del último comando, no antes ni durante. Es la
// ventana de la spec 09 §1(a) medida por orden, no supuesta.
func TestStepExecutable_LaEntradaSeEscribeDespuesDelUltimoComando(t *testing.T) {
	almacen := &entriesEspia{}
	ejecutable := domStep.NewStepExecutable(
		handlerQueAnota{orden: &almacen.orden}, &varsStoreSpy{}, almacen)

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

	require.Equal(t, []string{"comando", "escritura"}, almacen.orden)
}

// La entrada lleva la procedencia: quién la escribió y cuándo. Sin eso, «se
// salta porque ya está en caché» con una clave opaca deja sin respuesta la
// pregunta «¿cuándo se probó esto por última vez?» (spec 10 §5.4).
func TestStepExecutable_LaEntradaLlevaSuProcedenciaYSuCaducidad(t *testing.T) {
	almacen := &entriesEspia{}
	ejecutable := domStep.NewStepExecutable(handlerQueAnota{}, &varsStoreSpy{}, almacen)

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

	require.Len(t, almacen.entradas, 1)
	entrada := almacen.entradas[0]

	assert.Equal(t, "exec-1", entrada.ProducedBy.ExecutionID)
	assert.True(t, instanteDePrueba.Equal(entrada.ProducedBy.At),
		"el instante sale del reloj inyectable del agregado, no de time.Now()")
	require.NotNil(t, entrada.ExpiresAt)
	assert.True(t, instanteDePrueba.Add(domCache.DefaultTTL).Equal(*entrada.ExpiresAt))
}

// (b) de la spec 09 §1: si la escritura falla, el step no falla —el despliegue
// ocurrió— pero el usuario se entera. Antes el fallo del `Set` se convertía en
// una razón para ejecutar dentro de la propia decisión, y era indistinguible de
// «el código cambió».
func TestStepExecutable_UnFalloAlEscribirLaEntradaNoTumbaElStepPeroSeDice(t *testing.T) {
	almacen := &entriesEspia{putErr: errDeEscritura}
	emisor := &emisorEspia{}
	contexto := contextoDePruebaCon(t, emisor)

	ejecutable := domStep.NewStepExecutable(handlerQueAnota{}, &varsStoreSpy{}, almacen)

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
			handlerQueProduceVariable{}, almacen, &entriesEspia{})

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.NotZero(t, almacen.guardados)
	})

	t.Run("un step saltado no guarda nada", func(t *testing.T) {
		almacen := &varsStoreSpy{}
		entradas := &entriesEspia{}
		ejecutable := domStep.NewStepExecutable(handlerQueSalta{}, almacen, entradas)

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.Zero(t, almacen.guardados,
			"las variables declaradas estaban en el mapa acumulado, pero ningún comando las consumió")
		require.Empty(t, entradas.escritas,
			"ni la entrada de caché: no se ejecutó nada que justifique escribirla")
	})
}

// La limpieza del step corre aunque el step falle (spec 06 §5.1).
//
// `step_workdir` lo pone el `before` del step y lo retira su `after`, que hasta
// la spec 06 vivía detrás del camino feliz. El residuo no es cosmético: el mapa
// acumulado es el material del que sale la huella de variables —y con ella la
// `cache_key`, y mañana `content_id`— así que dejar ahí el workdir del step
// fallido es dejar residuo en la identidad.
func TestStepExecutable_UnStepFallidoNoDejaSuWorkdirEnElMapaAcumulado(t *testing.T) {
	t.Run("control: el step exitoso tampoco lo deja", func(t *testing.T) {
		contexto := contextoDePrueba(t)
		ejecutable := domStep.NewStepExecutable(handlerQueFalla{}, &varsStoreSpy{}, &entriesEspia{})

		require.NoError(t, ejecutable.Execute(contexto))

		_, presente := contexto.GetAccumulatedVar(command.VarStepWorkdir)
		require.False(t, presente)
	})

	t.Run("el step fallido tampoco", func(t *testing.T) {
		contexto := contextoDePrueba(t)
		ejecutable := domStep.NewStepExecutable(
			handlerQueFalla{err: errDelHandler}, &varsStoreSpy{}, &entriesEspia{})

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

// handlerQueAnota reproduce lo que hace el handler 04 cuando no hay entrada de
// caché: anota la clave y corre los comandos. Anotar no persiste nada —eso lo
// hace el camino de éxito de StepExecutable—, que es justo lo que estos tests
// miden.
type handlerQueAnota struct{ orden *[]string }

func (h handlerQueAnota) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	request.RecordCacheKey(claveAnotada())
	if h.orden != nil {
		*h.orden = append(*h.orden, "comando")
	}
	return nil
}

func (handlerQueAnota) SetNext(domStep.StepHandler) {}

// handlerQueAnotaYFalla es la muerte a mitad: la clave ya se anotó, los comandos
// empezaron y el step no llegó al final.
type handlerQueAnotaYFalla struct{ err error }

func (h handlerQueAnotaYFalla) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	request.RecordCacheKey(claveAnotada())
	return h.err
}

func (handlerQueAnotaYFalla) SetNext(domStep.StepHandler) {}

// handlerQueNoAnota es el paso cuyo material no se pudo componer: corre, termina
// bien y no deja clave.
type handlerQueNoAnota struct{}

func (handlerQueNoAnota) Handle(_ *context.Context, _ *domStep.StepRequestHandler) error {
	return nil
}

func (handlerQueNoAnota) SetNext(domStep.StepHandler) {}

func claveAnotada() domCache.CacheKey {
	key, err := domCache.NewCacheKey(materialDePrueba())
	if err != nil {
		panic(err) // el material es literal: un error aquí es un bug del test
	}
	return key
}

func materialDePrueba() domCache.Material {
	huella := func(version, digito string) fingerprint.Fingerprint {
		f, err := fingerprint.Parse(version + ":" + strings.Repeat(digito, 32))
		if err != nil {
			panic(err)
		}
		return f
	}
	return domCache.Material{
		Subject:      "https://vex.test/org/proyecto.git",
		Pipeline:     "https://vex.test/org/pipeline.git",
		Scope:        "prod",
		Step:         "supply",
		Instructions: huella(fingerprint.InstructionsVersion, "11"),
		Variables:    huella(fingerprint.VariablesVersion, "22"),
		Code:         huella(fingerprint.Version, "33"),
	}
}

func claveDePrueba(t *testing.T) domCache.CacheKey {
	t.Helper()
	key, err := domCache.NewCacheKey(materialDePrueba())
	require.NoError(t, err)
	return key
}

// ── Espía del almacén de caché ──────────────────────────────────────────────
//
// Cuenta escrituras y su orden respecto de los comandos: cero al consultar, y
// exactamente una después de que el último comando terminó.

type entriesEspia struct {
	escritas []string
	entradas []domCache.Entry
	putErr   error
	orden    []string
}

var _ domCache.Entries = (*entriesEspia)(nil)

func (e *entriesEspia) Get(*context.Context, domCache.CacheKey) (domCache.Entry, bool, error) {
	return domCache.Entry{}, false, nil
}

func (e *entriesEspia) Put(_ *context.Context, key domCache.CacheKey, entry domCache.Entry) error {
	e.orden = append(e.orden, "escritura")
	if e.putErr != nil {
		return e.putErr
	}
	e.escritas = append(e.escritas, key.String())
	e.entradas = append(e.entradas, entry)
	return nil
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

var instanteDePrueba = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

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
		shared.NewFixedClock(instanteDePrueba),
	)

	executionContext := command.NewExecutionContext(&ctx, ejecucion, nil, nil, emisor, nil)

	stepName, err := command.NewStepName("02-supply")
	require.NoError(t, err)
	executionContext.SetStepName(stepName)
	executionContext.SetWorkdir(t.TempDir())

	return executionContext
}
