package step_test

// El bucle de decisión de la spec 15 §5.6, con el handler entero.
//
// Lo que se prueba aquí es lo que ni `rule_test.go` ni el harness pueden ver:
// que el handler evalúe EXACTAMENTE lo declarado —ni una regla más ni una
// menos—, que el motivo que emite corresponda a la regla que lo cerró, y que la
// advertencia de §5.4 salga de la carga de la configuración y no aborte nada.
//
// El bucle lo dejó montado la spec 11; lo que la 15 sustituye son los dos `if`
// que había dentro.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	domNotify "github.com/jairoprogramador/vex-engine/old-internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/shared"
	domState "github.com/jairoprogramador/vex-engine/old-internal/domain/state"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
)

const huellaDelArbolDePrueba = "v1:4444444444444444444444444444444444444444444444444444444444444444"

// stepDePrueba es el directorio del step, con su prefijo de orden: es la clave
// con la que el material se carga y se consume.
const stepDePrueba = "02-supply"

// ── El montaje ──────────────────────────────────────────────────────────────

// decidir corre el handler 03 contra la configuración declarada y el último
// registro que se le dé, y devuelve las líneas emitidas más si los comandos
// llegaron a correr.
type decision struct {
	ejecutado bool
	lineas    []string
	huella    string
}

func (d decision) dice(fragmento string) bool {
	for _, linea := range d.lineas {
		if strings.Contains(linea, fragmento) {
			return true
		}
	}
	return false
}

func decidir(
	t *testing.T,
	rules domStep.RuleSet,
	ultimo *domState.StepRecord,
) decision {
	t.Helper()

	emisor := &emisorEspia{}
	ejecutable := &ejecutableEspia{}
	contexto := contextoConEjecutable(t, emisor, ejecutable)
	contexto.SetProjectStatus(huellaDelArbolDePrueba)

	config, err := domStep.NewStepConfig(domStep.NewEnvironmentScope(), rules)
	require.NoError(t, err)

	registros := &registrosConUltimo{}
	if ultimo != nil {
		registros.ultimo, registros.hay = *ultimo, true
	}

	handler := domStep.NewStepRunnerHandler(materialCargado(t, config), domStep.NewLastRecordProvider(registros))

	request := domStep.NewStepRequestHandler(contexto, contexto.StepName())
	require.NoError(t, handler.Handle(request.Ctx(), request))

	return decision{
		ejecutado: ejecutable.veces > 0,
		lineas:    emisor.lineas,
		huella:    request.StepFingerprint(),
	}
}

// registroCon compone el último registro de la clave: la huella con la que se
// escribió y hace cuánto.
func registroCon(t *testing.T, fingerprint string, edad time.Duration) *domState.StepRecord {
	t.Helper()
	at := instanteDePrueba.Add(-edad)
	id, err := domState.NewRecordID(at, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.NoError(t, err)
	record, err := domState.NewStepRecord(id, fingerprint, nil,
		domState.Provenance{ExecutionID: "exec-0", At: at})
	require.NoError(t, err)
	return &record
}

// huellaVigente es la que este montaje produce: se obtiene decidiendo sin
// registro previo y leyendo la que el handler anotó.
func huellaVigente(t *testing.T, rules domStep.RuleSet) string {
	t.Helper()
	huella := decidir(t, rules, nil).huella
	require.NotEmpty(t, huella)
	return huella
}

func reglas(t *testing.T, rules ...domStep.Rule) domStep.RuleSet {
	t.Helper()
	set, err := domStep.NewRuleSet(rules...)
	require.NoError(t, err)
	return set
}

func maxAgeDe(t *testing.T, texto string) domStep.Rule {
	t.Helper()
	rule, err := domStep.NewMaxAgeRule(texto)
	require.NoError(t, err)
	return rule
}

// ── Los casos ───────────────────────────────────────────────────────────────

// Se evalúa EXACTAMENTE lo declarado. `state_changed` no es un chequeo
// obligatorio: un step que no lo declara puede revivir su resultado anterior
// aunque su huella haya cambiado. No existe una comprobación que el motor
// imponga por fuera de lo que el `config.yaml` dice.
func TestStepRunnerHandler_SoloSeEvaluaLoDeclarado(t *testing.T) {
	stateChanged := reglas(t, domStep.NewDefaultStateChangedRule())
	vigente := huellaVigente(t, stateChanged)

	t.Run("state_changed revive con la misma huella, por vieja que sea", func(t *testing.T) {
		d := decidir(t, stateChanged, registroCon(t, vigente, 400*24*time.Hour))

		assert.False(t, d.ejecutado,
			"sin `max_age` no caduca: el TTL global de 30 días se retiró con §5.7")
	})

	t.Run("state_changed se ejecuta con otra huella", func(t *testing.T) {
		d := decidir(t, stateChanged, registroCon(t, huellaA, time.Minute))

		require.True(t, d.ejecutado)
		assert.True(t, d.dice("el contenido del step cambió"))
	})

	t.Run("sólo max_age revive aunque la huella haya cambiado", func(t *testing.T) {
		soloExpira := reglas(t, maxAgeDe(t, "24h"))

		d := decidir(t, soloExpira, registroCon(t, huellaA, time.Hour))

		assert.False(t, d.ejecutado,
			"la huella es OTRA y ninguna regla declarada la mira: de esto avisa §5.4")
	})

	t.Run("sólo max_age se ejecuta al salir de la ventana", func(t *testing.T) {
		soloExpira := reglas(t, maxAgeDe(t, "1h"))

		d := decidir(t, soloExpira, registroCon(t, huellaA, 2*time.Hour))

		require.True(t, d.ejecutado)
		assert.True(t, d.dice("ha caducado"))
	})
}

// Un step que no declara `state_changed` no compone huella, y su registro sale
// SIN ella. No es una carencia: es el mecanismo que ya existía para «nunca
// revive», y anotar una huella que ninguna regla suya sostiene sería dejar en el
// registro una afirmación que nadie hizo.
func TestStepRunnerHandler_LaHuellaSeComponeSoloSiAlguienLaMira(t *testing.T) {
	conInvalidacion := decidir(t, reglas(t, domStep.NewDefaultStateChangedRule()), nil)
	assert.NotEmpty(t, conInvalidacion.huella)

	soloExpiracion := decidir(t, reglas(t, maxAgeDe(t, "24h")), nil)
	assert.Empty(t, soloExpiracion.huella)

	sinReglas := decidir(t, domStep.EmptyRuleSet(), nil)
	assert.Empty(t, sinReglas.huella)
}

// Ausencia de registro ⇒ ejecutar, aunque ninguna regla se cumpla por falta de
// con qué comparar. Lo que falta no es una regla que se cumpla: es el sujeto
// sobre el que se evalúan.
func TestStepRunnerHandler_SinRegistroSeEjecuta(t *testing.T) {
	for _, caso := range []struct {
		nombre string
		rules  domStep.RuleSet
	}{
		{"con state_changed", reglas(t, domStep.NewDefaultStateChangedRule())},
		{"con sólo max_age", reglas(t, maxAgeDe(t, "24h"))},
	} {
		t.Run(caso.nombre, func(t *testing.T) {
			d := decidir(t, caso.rules, nil)

			require.True(t, d.ejecutado)
			assert.True(t, d.dice("no consta que este step se haya ejecutado aquí"))
		})
	}
}

// §5.5 desde el handler: sin reglas se ejecuta, y el motivo lo dice. La otra
// mitad —que tampoco escriba registro— está en `step_executable_test.go`.
func TestStepRunnerHandler_SinReglasSeEjecutaConMotivoPropio(t *testing.T) {
	d := decidir(t, domStep.EmptyRuleSet(), registroCon(t, huellaA, time.Minute))

	require.True(t, d.ejecutado)
	assert.True(t, d.dice("no declara reglas de re-ejecución"))
	assert.False(t, d.dice("cambió"),
		"no se re-ejecuta POR una regla, sino por su ausencia: no se disfraza de otra cosa")
}

// LA ADVERTENCIA DE §5.4, y las dos mitades importan: se emite, y la ejecución
// CONTINÚA. No pasa por el validador de la spec 04 porque su veredicto es un
// `error` y sólo sabe abortar.
func TestStepRunnerHandler_ExpirarSinInvalidarAvisaYNoAborta(t *testing.T) {
	t.Run("sólo expiración: avisa", func(t *testing.T) {
		d := decidir(t, reglas(t, maxAgeDe(t, "24h")), nil)

		assert.True(t, d.dice("sólo declara reglas de expiración"))
		assert.True(t, d.dice("state_changed"), "el aviso dice qué falta")
		assert.True(t, d.ejecutado, "y no aborta: es una elección válida, no un error")
	})

	t.Run("con invalidación presente: no avisa", func(t *testing.T) {
		d := decidir(t, reglas(t, domStep.NewDefaultStateChangedRule(), maxAgeDe(t, "24h")), nil)

		assert.False(t, d.dice("sólo declara reglas de expiración"))
	})

	t.Run("sin reglas: tampoco avisa", func(t *testing.T) {
		d := decidir(t, domStep.EmptyRuleSet(), nil)

		assert.False(t, d.dice("sólo declara reglas de expiración"),
			"no hay ninguna vigencia de la que fiarse: el step se ejecuta siempre")
	})
}

// El fail-open de la spec 09 §5.3 se conserva, y sigue sin ser silencioso: «no
// pude componer la huella» ejecuta como «el contenido cambió», pero NO se
// disfraza de ello.
func TestStepRunnerHandler_SinMaterialSeEjecutaYSeDice(t *testing.T) {
	emisor := &emisorEspia{}
	contexto := contextoConEjecutable(t, emisor, &ejecutableEspia{})
	contexto.SetProjectStatus("no-es-una-huella")

	config, err := domStep.NewStepConfig(
		domStep.NewEnvironmentScope(), reglas(t, domStep.NewDefaultStateChangedRule()))
	require.NoError(t, err)

	handler := domStep.NewStepRunnerHandler(materialCargado(t, config), domStep.NewLastRecordProvider(&registrosConUltimo{}))
	request := domStep.NewStepRequestHandler(contexto, contexto.StepName())

	require.NoError(t, handler.Handle(request.Ctx(), request))

	assert.Empty(t, request.StepFingerprint(),
		"sin huella el registro se escribe igual, y no revivirá nunca")
	assert.True(t, emisor.contiene("no se pudo componer la huella"))
	assert.True(t, emisor.contiene("no se pudo determinar si ya se ejecutó"))
	assert.False(t, emisor.contiene("el contenido del step cambió"),
		"la duda viaja en su propio motivo: confundirla con un cambio era el defecto")
}

// ── Dobles ──────────────────────────────────────────────────────────────────

// materialCargado es lo que el resolutor de la cadena de pipeline dejó cargado
// para este step (spec 18 §5.2). Sustituye a los dos repositorios que el handler
// tenía inyectados: desde la 18 no lee del disco, consume.
// materialCargado es lo que el resolutor de la cadena de pipeline deja para la
// cadena de step, huella `pipe-v1` incluida (spec 27 §5.2).
//
// La declaración llega YA COMPUESTA: es la propiedad que este handler estrena —la
// huella deja de depender del mapa acumulado y se conoce antes de abrir el step—
// y por eso el montaje puede darla como un valor fijo.
func materialCargado(t *testing.T, config domStep.StepConfig) *domStep.LoadedPipelinecode {
	t.Helper()

	declaracion, err := domFingerprint.Parse(
		domFingerprint.DeclarationVersion + ":" + strings.Repeat("5", 64))
	require.NoError(t, err)

	cargado := domStep.NewLoadedPipelinecode()
	cargado.Put(stepDePrueba, domStep.LoadedStep{
		Commands:    []command.Command{comandoDePrueba(t)},
		Config:      config,
		Declaration: declaracion,
	})
	return cargado
}

// registrosConUltimo devuelve el último registro que se le ponga. El puerto sólo
// tiene `Last` y `Append`: no hay `Delete` ni `Update`, y eso es el tipo diciendo
// la regla de vida del almacén (spec 11).
type registrosConUltimo struct {
	ultimo domState.StepRecord
	hay    bool
}

var _ domState.Records = (*registrosConUltimo)(nil)

// Get no participa de la decisión de re-ejecutar: la regla mira el ÚLTIMO
// registro, nunca uno concreto.
func (r *registrosConUltimo) Get(*context.Context, domState.Key, domState.RecordID) (domState.StepRecord, bool, error) {
	return domState.StepRecord{}, false, nil
}

func (r *registrosConUltimo) Last(*context.Context, domState.Key) (domState.StepRecord, bool, error) {
	return r.ultimo, r.hay, nil
}

func (r *registrosConUltimo) Append(*context.Context, domState.Key, domState.StepRecord) error {
	return nil
}

type ejecutableEspia struct{ veces int }

var _ command.Executable = (*ejecutableEspia)(nil)

func (e *ejecutableEspia) Execute(*command.ExecutionContext) error {
	e.veces++
	return nil
}

// contextoConEjecutable es el contexto de prueba con un ejecutable de comando
// puesto: el handler 03 sí los corre, a diferencia de los dobles de
// `step_executable_test.go`.
func contextoConEjecutable(
	t *testing.T,
	emisor domNotify.LogObserver,
	ejecutable command.Executable) *command.ExecutionContext {

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

	executionContext := command.NewExecutionContext(&ctx, ejecucion, ejecutable, nil, emisor, nil)

	stepName, err := command.NewStepName(stepDePrueba)
	require.NoError(t, err)
	executionContext.SetStepName(stepName)
	executionContext.SetWorkdir(t.TempDir())

	return executionContext
}
