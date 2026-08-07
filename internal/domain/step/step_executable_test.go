package step_test

// El estado se escribe UNA sola vez, desde aquí, y solo si el step terminó bien
// (spec 09 §5.2, heredado por la 10 §5.2 y por la 11 §5.3).
//
// Aquí vivía el borrado compensatorio: las reglas escribían la huella nueva
// dentro de `Evaluate` —antes del primer comando— y este `Delete` la revertía si
// el step fallaba. La spec 09 lo eliminó junto con su causa; la 10 no cambió el
// MOMENTO —eso ya estaba— sino QUÉ se escribe; la 11 cambia la REGLA DE VIDA de
// lo escrito: un registro nuevo que no sustituye a ninguno, más un puntero de
// índice hacia él.

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
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

var (
	errDelHandler  = errors.New("el comando salió con 1")
	errDeEscritura = errors.New("permiso denegado al escribir el estado")
)

func TestStepExecutable_ElRegistroSeEscribeSoloTrasElExito(t *testing.T) {
	t.Run("un step exitoso deja UN registro, bajo el ámbito que declara", func(t *testing.T) {
		// Aquí se escribían DOS —uno de proyecto y otro de ambiente— porque un
		// step podía producir de los dos ámbitos y no había forma de saber cuál
		// era el suyo. Con el ámbito declarado queda uno (spec 13 §5.4).
		for _, caso := range []struct {
			scope  domStep.Scope
			ambito string
		}{
			{domStep.NewEnvironmentScope(), "environment:prod"},
			{domStep.NewProjectScope(), "project"},
		} {
			t.Run(caso.scope.String(), func(t *testing.T) {
				registros := &recordsEspia{}
				indice := &entriesEspia{}
				ejecutable := nuevoEjecutable(handlerQueAnota{scope: caso.scope}, registros, indice)

				require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

				require.Equal(t, []string{caso.ambito}, registros.ambitos())
				assert.Equal(t, []string{claveDePrueba(t).String()}, indice.escritas)
			})
		}
	})

	// La otra mitad de §5.3, y la que es fácil de perder: sin `config.yaml` el
	// step corre y NO deja nada. Inventarle `environment` por defecto sería la
	// deducción implícita que esta spec retira, sólo que en el otro archivo.
	t.Run("un step SIN config.yaml no deja registro ni entrada", func(t *testing.T) {
		registros := &recordsEspia{}
		indice := &entriesEspia{}
		ejecutable := nuevoEjecutable(handlerQueAnotaSinAmbito{}, registros, indice)

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

		assert.Empty(t, registros.anadidos,
			"no declara ámbito, así que no hay dónde recordar que corrió")
		assert.Empty(t, indice.escritas,
			"y el índice apunta a registros: sin registro no hay a qué apuntar")
	})

	// Una muerte dura es, vista desde el código, «se decidió ejecutar y no se
	// llegó al camino de éxito». Antes la huella ya estaba escrita para entonces
	// —la escribía `Evaluate`— y solo el borrado compensatorio la quitaba, que es
	// código que corre después y que un SIGKILL no ejecuta.
	t.Run("un step que anota su huella y NO termina no deja nada", func(t *testing.T) {
		registros := &recordsEspia{}
		indice := &entriesEspia{}
		ejecutable := nuevoEjecutable(
			handlerQueAnotaYFalla{err: errDelHandler}, registros, indice)

		require.ErrorIs(t, ejecutable.Execute(contextoDePrueba(t)), errDelHandler)
		require.Empty(t, registros.anadidos, "no hay registro de «se intentó»")
		require.Empty(t, indice.escritas)
	})

	// «Uno por ejecución REAL del step — nunca cuando se revive» (spec 11 §5.3).
	//
	// No es contable: el step que revive no anota huella, así que su registro
	// saldría con la huella vacía y dejaría al step sin poder revivir NUNCA más.
	// El síntoma es una oscilación de periodo dos —ejecuta, revive, ejecuta,
	// revive— y su red a nivel de integración es
	// `TestRunCommand_ReejecucionSinCambios` a partir de la tercera corrida.
	t.Run("un step que revive no deja registro", func(t *testing.T) {
		registros := &recordsEspia{}
		indice := &entriesEspia{}
		ejecutable := nuevoEjecutable(handlerQueRevive{}, registros, indice)

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.Empty(t, registros.anadidos,
			"revivir no es un hecho nuevo del step: es la constatación de uno viejo")
		require.Empty(t, indice.escritas)
	})

	t.Run("un step saltado no deja nada", func(t *testing.T) {
		registros := &recordsEspia{}
		indice := &entriesEspia{}
		ejecutable := nuevoEjecutable(handlerQueSalta{}, registros, indice)

		require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))
		require.Empty(t, registros.anadidos,
			"las variables declaradas estaban en el mapa acumulado, pero ningún comando las consumió")
		require.Empty(t, indice.escritas)
	})
}

// EL cambio de la spec 11, y la asimetría de su §4: un step cuyo material no se
// pudo componer SÍ deja registro —lo que produjo es estado real, y perder de
// vista un ARN es peor que re-ejecutar— pero sin huella, así que no revivirá
// nunca. Lo que no deja es entrada de índice: no hay contenido bajo el que
// indexarlo.
func TestStepExecutable_UnStepSinHuellaRegistraIgualPeroNoIndexa(t *testing.T) {
	registros := &recordsEspia{}
	indice := &entriesEspia{}
	ejecutable := nuevoEjecutable(handlerQueNoAnota{}, registros, indice)

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

	require.Len(t, registros.anadidos, 1)
	assert.Empty(t, registros.anadidos[0].registro.StepFingerprint())
	assert.False(t, registros.anadidos[0].registro.Revives(""))
	assert.Empty(t, indice.escritas, "un puntero bajo una clave incompleta colisionaría")
}

// La escritura ocurre DESPUÉS del último comando, no antes ni durante. Es la
// ventana de la spec 09 §1(a) medida por orden, no supuesta.
func TestStepExecutable_ElRegistroSeEscribeDespuesDelUltimoComando(t *testing.T) {
	registros := &recordsEspia{}
	ejecutable := nuevoEjecutable(
		handlerQueAnota{orden: &registros.orden}, registros, &entriesEspia{})

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

	require.Equal(t, []string{"comando", "escritura"}, registros.orden)
}

// El registro lleva su procedencia: quién lo escribió y cuándo. Sin eso, «se
// revive» dejaría sin respuesta «¿cuándo se probó esto por última vez?», y el
// rollback de la spec 28 no tendría a qué anclar.
func TestStepExecutable_ElRegistroLlevaSuProcedenciaYSuHuella(t *testing.T) {
	registros := &recordsEspia{}
	ejecutable := nuevoEjecutable(handlerQueAnota{}, registros, &entriesEspia{})

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

	require.NotEmpty(t, registros.anadidos)
	registro := registros.anadidos[0].registro

	assert.Equal(t, "exec-1", registro.ProducedBy().ExecutionID)
	assert.True(t, instanteDePrueba.Equal(registro.ProducedBy().At),
		"el instante sale del reloj inyectable del agregado, no de time.Now()")
	assert.Equal(t, claveDePrueba(t).String(), registro.StepFingerprint())
	assert.False(t, registro.ID().IsZero())
}

// El registro lleva TODO lo no volátil del mapa acumulado, sin repartir: el
// ámbito ya no es un atributo de cada variable, así que no hay por dónde
// partirlo (spec 13 §5.6).
//
// Este test se llamaba `...SeParteEnDosAmbitosYFiltraLasVolatiles` y afirmaba
// que `artifact_url` —marcada compartida— iba a un registro y `acr_name` a otro.
// Lo que se conserva entero es el filtro de volátiles: las seis las deriva el
// motor de la ejecución en curso, así que guardarlas sería guardar basura que la
// corrida siguiente recalcularía distinta.
func TestStepExecutable_ElRegistroLlevaElMapaAcumuladoSinLasVolatiles(t *testing.T) {
	registros := &recordsEspia{}
	ejecutable := nuevoEjecutable(handlerQueProduceVariables{}, registros, &entriesEspia{})

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

	require.Len(t, registros.anadidos, 1)
	assert.Equal(t, []string{"acr_name", "artifact_url"}, nombresDe(registros.anadidos[0].registro),
		"las dos van al mismo sitio: el ámbito lo declara el step, no la variable")

	for _, nombre := range command.VolatileVarNames() {
		assert.NotContains(t, nombresDe(registros.anadidos[0].registro), nombre)
	}
}

// (b) de la spec 09 §1: si la escritura falla, el step no falla —el despliegue
// ocurrió— pero el usuario se entera. Lo que se pierde aquí es un HECHO, no una
// optimización, así que el mensaje lo dice en esos términos.
func TestStepExecutable_UnFalloAlRegistrarNoTumbaElStepPeroSeDice(t *testing.T) {
	registros := &recordsEspia{appendErr: errDeEscritura}
	emisor := &emisorEspia{}
	contexto := contextoDePruebaCon(t, emisor)

	ejecutable := nuevoEjecutable(handlerQueAnota{}, registros, &entriesEspia{})

	require.NoError(t, ejecutable.Execute(contexto),
		"no haber podido guardar el estado no invalida el despliegue que sí ocurrió")
	assert.True(t, emisor.contiene("no se pudo registrar el estado del step"),
		"pero deja de ser silencioso: se acaba de perder la pista de un recurso real")
	assert.False(t, emisor.contiene("cambió"),
		"y no se disfraza de cambio de contenido")
}

// Si el registro no se pudo escribir, el índice no apunta a él: un índice con
// punteros rotos dejaría de ser reconstruible sin distinguir cuáles lo están.
func TestStepExecutable_SinRegistroNoHayIndice(t *testing.T) {
	indice := &entriesEspia{}
	ejecutable := nuevoEjecutable(
		handlerQueAnota{}, &recordsEspia{appendErr: errDeEscritura}, indice)

	require.NoError(t, ejecutable.Execute(contextoDePrueba(t)))

	assert.Empty(t, indice.escritas)
}

// El índice no decide nada, así que un fallo suyo no puede sonar como el del
// almacén de registros.
func TestStepExecutable_UnFalloAlIndexarNoTumbaElStepPeroSeDice(t *testing.T) {
	emisor := &emisorEspia{}
	contexto := contextoDePruebaCon(t, emisor)
	ejecutable := nuevoEjecutable(
		handlerQueAnota{}, &recordsEspia{}, &entriesEspia{putErr: errDeEscritura})

	require.NoError(t, ejecutable.Execute(contexto))
	assert.True(t, emisor.contiene("no se pudo indexar el registro"))
}

// La limpieza del step corre aunque el step falle (spec 06 §5.1).
//
// `step_workdir` lo pone el `before` del step y lo retira su `after`, que hasta
// la spec 06 vivía detrás del camino feliz. El residuo no es cosmético: el mapa
// acumulado es el material del que sale la huella de variables —y con ella la
// huella del step, y mañana `content_id`— así que dejar ahí el workdir del step
// fallido es dejar residuo en la identidad.
func TestStepExecutable_UnStepFallidoNoDejaSuWorkdirEnElMapaAcumulado(t *testing.T) {
	t.Run("control: el step exitoso tampoco lo deja", func(t *testing.T) {
		contexto := contextoDePrueba(t)
		ejecutable := nuevoEjecutable(handlerQueFalla{}, &recordsEspia{}, &entriesEspia{})

		require.NoError(t, ejecutable.Execute(contexto))

		_, presente := contexto.GetAccumulatedVar(command.VarStepWorkdir)
		require.False(t, presente)
	})

	t.Run("el step fallido tampoco", func(t *testing.T) {
		contexto := contextoDePrueba(t)
		ejecutable := nuevoEjecutable(
			handlerQueFalla{err: errDelHandler}, &recordsEspia{}, &entriesEspia{})

		require.ErrorIs(t, ejecutable.Execute(contexto), errDelHandler)

		_, presente := contexto.GetAccumulatedVar(command.VarStepWorkdir)
		require.False(t, presente,
			"el workdir del step que falló seguiría contaminando la identidad del siguiente")
	})
}

// ── Dobles ──────────────────────────────────────────────────────────────────

func nuevoEjecutable(
	handler domStep.StepHandler,
	registros domState.Records,
	indice domCache.Entries) *domStep.StepExecutable {

	return domStep.NewStepExecutable(handler, registros, idsFijos{}, indice)
}

type handlerQueFalla struct{ err error }

func (h handlerQueFalla) Handle(_ *context.Context, _ *domStep.StepRequestHandler) error {
	return h.err
}

func (h handlerQueFalla) SetNext(domStep.StepHandler) {}

// declararAmbito reproduce lo que hace el handler 03 tras leer el `config.yaml`
// del step. Sin esta llamada el step no declara ámbito y no persiste nada, que
// es el default seguro de la spec 13 §5.3.
func declararAmbito(request *domStep.StepRequestHandler, scope domStep.Scope) error {
	config, err := domStep.NewStepConfig(scope)
	if err != nil {
		return err
	}
	request.SetStepConfig(config)
	return nil
}

// handlerQueProduceVariables deja dos variables en el mapa acumulado, que es lo
// que el registro persiste al terminar el step.
type handlerQueProduceVariables struct{}

func (handlerQueProduceVariables) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	if err := declararAmbito(request, domStep.NewEnvironmentScope()); err != nil {
		return err
	}
	primera, err := command.NewVariable("acr_name", "vexsand-demo-app", command.OriginRuntime)
	if err != nil {
		return err
	}
	segunda, err := command.NewVariable("artifact_url", "s3://artefactos/demo", command.OriginRuntime)
	if err != nil {
		return err
	}
	request.AddAccumulatedVars(primera)
	request.AddAccumulatedVars(segunda)
	request.MarkStepExecuted()
	return nil
}

func (handlerQueProduceVariables) SetNext(domStep.StepHandler) {}

// handlerQueSalta reproduce lo que hace el handler 04 con un commands.yaml vacío:
// las variables declaradas ya están en el mapa, pero ningún comando corrió.
type handlerQueSalta struct{}

func (handlerQueSalta) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	if err := declararAmbito(request, domStep.NewEnvironmentScope()); err != nil {
		return err
	}
	variable, err := command.NewVariable("acr_name", "vexsand-demo-app", command.OriginDeclared)
	if err != nil {
		return err
	}
	request.AddAccumulatedVars(variable)
	request.MarkStepSkipped(domStep.SkipReasonNoCommands)
	return nil
}

func (handlerQueSalta) SetNext(domStep.StepHandler) {}

// handlerQueRevive reproduce lo que hace el handler 04 cuando el último registro
// de la clave dice que este trabajo ya está hecho: no marca ejecución, no anota
// huella y termina bien. Ni siquiera es un `skipped`: el step está al día.
type handlerQueRevive struct{}

func (handlerQueRevive) Handle(_ *context.Context, _ *domStep.StepRequestHandler) error {
	return nil
}

func (handlerQueRevive) SetNext(domStep.StepHandler) {}

// handlerQueAnota reproduce lo que hace el handler 03 cuando no hay registro
// vigente: lee el ámbito, anota la huella y corre los comandos. Anotar no
// persiste nada —eso lo hace el camino de éxito de StepExecutable—, que es justo
// lo que estos tests miden.
//
// El ámbito por defecto es el del ambiente, que es lo que declara el fixture; el
// campo existe para el caso que declara `project`.
type handlerQueAnota struct {
	orden *[]string
	scope domStep.Scope
}

func (h handlerQueAnota) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	scope := h.scope
	if scope.IsZero() {
		scope = domStep.NewEnvironmentScope()
	}
	if err := declararAmbito(request, scope); err != nil {
		return err
	}
	request.RecordStepFingerprint(huellaAnotada())
	request.MarkStepExecuted()
	if h.orden != nil {
		*h.orden = append(*h.orden, "comando")
	}
	return nil
}

func (handlerQueAnota) SetNext(domStep.StepHandler) {}

// handlerQueAnotaSinAmbito es el step sin `config.yaml`: corre, termina bien y
// no tiene dónde recordarse (spec 13 §5.3).
type handlerQueAnotaSinAmbito struct{}

func (handlerQueAnotaSinAmbito) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	request.MarkStepExecuted()
	return nil
}

func (handlerQueAnotaSinAmbito) SetNext(domStep.StepHandler) {}

// handlerQueAnotaYFalla es la muerte a mitad: la huella ya se anotó, los comandos
// empezaron y el step no llegó al final.
type handlerQueAnotaYFalla struct{ err error }

func (h handlerQueAnotaYFalla) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	if err := declararAmbito(request, domStep.NewEnvironmentScope()); err != nil {
		return err
	}
	request.RecordStepFingerprint(huellaAnotada())
	request.MarkStepExecuted()
	return h.err
}

func (handlerQueAnotaYFalla) SetNext(domStep.StepHandler) {}

// handlerQueNoAnota es el step cuyo material no se pudo componer: corre, termina
// bien y no deja huella. Declara ámbito: lo que le falta es la huella, no el
// sitio donde guardar.
type handlerQueNoAnota struct{}

func (handlerQueNoAnota) Handle(_ *context.Context, request *domStep.StepRequestHandler) error {
	if err := declararAmbito(request, domStep.NewEnvironmentScope()); err != nil {
		return err
	}
	request.MarkStepExecuted()
	return nil
}

func (handlerQueNoAnota) SetNext(domStep.StepHandler) {}

func huellaAnotada() domCache.CacheKey {
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

// idsFijos hace reproducible el ULID: la aleatoriedad es infraestructura, y con
// el puerto se puede fijar entera.
type idsFijos struct{}

func (idsFijos) New(at time.Time) (domState.RecordID, error) {
	return domState.NewRecordID(at, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
}

// ── Espías de las dos tiendas ───────────────────────────────────────────────

type anadido struct {
	clave    domState.Key
	registro domState.StepRecord
}

type recordsEspia struct {
	anadidos  []anadido
	appendErr error
	orden     []string
}

var _ domState.Records = (*recordsEspia)(nil)

func (r *recordsEspia) Last(*context.Context, domState.Key) (domState.StepRecord, bool, error) {
	return domState.StepRecord{}, false, nil
}

func (r *recordsEspia) Append(_ *context.Context, key domState.Key, record domState.StepRecord) error {
	r.orden = append(r.orden, "escritura")
	if r.appendErr != nil {
		return r.appendErr
	}
	r.anadidos = append(r.anadidos, anadido{clave: key, registro: record})
	return nil
}

func (r *recordsEspia) ambitos() []string {
	out := make([]string, 0, len(r.anadidos))
	for _, a := range r.anadidos {
		out = append(out, a.clave.Scope().String())
	}
	return out
}

func nombresDe(record domState.StepRecord) []string {
	variables := record.Variables()
	out := make([]string, 0, len(variables))
	for i := range variables {
		out = append(out, variables[i].Name())
	}
	return out
}

type entriesEspia struct {
	escritas []string
	entradas []domCache.Entry
	putErr   error
}

var _ domCache.Entries = (*entriesEspia)(nil)

func (e *entriesEspia) Get(*context.Context, domCache.CacheKey) (domCache.Entry, bool, error) {
	return domCache.Entry{}, false, nil
}

func (e *entriesEspia) Put(_ *context.Context, key domCache.CacheKey, entry domCache.Entry) error {
	if e.putErr != nil {
		return e.putErr
	}
	e.escritas = append(e.escritas, key.String())
	e.entradas = append(e.entradas, entry)
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
