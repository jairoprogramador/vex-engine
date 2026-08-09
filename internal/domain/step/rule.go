package step

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// ruleFieldSep separa la clave de una regla de su parametrización dentro de la
// forma canónica. Es el mismo separador que usa `VariableDeclaration.Canonical`
// —un byte que no aparece en un YAML escrito por una persona— y por la misma
// razón: dos reglas distintas no pueden producir la misma cadena por
// concatenación.
const ruleFieldSep = declarationFieldSep

// Las reglas de re-ejecución: qué invalida el trabajo que un step ya hizo
// (spec 15).
//
// Hasta aquí la respuesta era una sola para todos los steps —«todo importa
// siempre», más un TTL de 30 días que el motor imponía— y era segura y cara: un
// step que crea un registro de contenedores no depende del código de la
// aplicación, y aun así se re-ejecutaba en cada commit.
//
// Lo que se mueve es de quién es la política. El motor conserva el
// VOCABULARIO —qué reglas existen y qué significa cada una— y pierde la
// SELECCIÓN, que pasa al `config.yaml` del step. Es el mismo movimiento que la
// spec 13 hace con el ámbito, y por la misma razón: el motor no puede saber qué
// le importa a un step que no conoce.
//
// # Las dos categorías, y son lenguaje antes que código
//
// Toda regla pertenece a UNA de estas dos —o a una tercera, ANULACIÓN
// EXPLÍCITA, si algún día se añade la capacidad de forzar una re-ejecución a
// mano—, y declararla es obligatorio: `Category()` está en la interfaz para que
// la tercera regla que alguien añada tenga que elegir, en vez de aparecer como
// un caso más de un `switch`.
//
//	INVALIDACIÓN  afirma que lo guardado ya NO ES CORRECTO
//	EXPIRACIÓN    fuerza una revisión periódica, sin afirmar que algo cambió
//
// Mezclarlas fue lo que produjo el TTL global: cuando la única forma de decir
// «revísalo de vez en cuando» es la misma que dice «esto ya no vale», la
// política más conservadora gana y se aplica a todo. Nombrarlas es lo que
// permite que un step pida una sin la otra — y lo que hace expresable la
// advertencia de §5.4, que es exactamente «expiración sin invalidación».
type RuleCategory string

const (
	// CategoryInvalidation — lo guardado ya no es correcto.
	CategoryInvalidation RuleCategory = "invalidación"

	// CategoryExpiration — lo guardado lleva demasiado tiempo sin revisarse.
	CategoryExpiration RuleCategory = "expiración"
)

// RuleKind es la clave con la que una regla se declara en `rules`, y el
// vocabulario es CERRADO: una regla que el motor no conoce es un error de
// pipelinecode, no una regla que se ignora.
//
// Ignorarla sería la peor de las dos opciones: quien la escribió cree que su
// step tiene una comprobación que no tiene, y el motor no puede decirle que no.
type RuleKind string

const (
	// RuleKindNone es la ausencia de regla: lo que devuelve el combinador cuando
	// ninguna se cumplió, y cuando el conjunto está vacío.
	RuleKindNone RuleKind = ""

	RuleKindStateChanged RuleKind = "state_changed"
	RuleKindMaxAge       RuleKind = "max_age"
)

// Las dos fuentes que `state_changed` puede vigilar (spec 15 §5.2).
//
//	pipeline  la declaración del step: su `commands.yaml`, su `config.yaml` y
//	          el árbol de su directorio. NO se puede omitir.
//	project   la huella del árbol del código a desplegar (spec 08). Sí.
const (
	StateSourcePipeline = "pipeline"
	StateSourceProject  = "project"
)

// RuleSubject es todo lo que una regla puede mirar, y no hay más: la huella
// actual del step, su último registro y el instante de la corrida.
//
// **Ningún campo es `deployment_id`, `content_id` ni `parent`.** Esa
// independencia es intencional y es la propiedad central del diseño: son
// identificadores que responden otras preguntas, y `deployment_id` además cambia
// en cada ejecución nueva al mismo ambiente aunque el step no haya cambiado
// nada. Que no estén en el tipo es más fuerte que documentar que no se usan.
//
// El instante llega como dato, no del reloj: `time.Now()` está prohibido en el
// dominio (spec 07 §5.1) y es lo que permite probar los dos lados del borde de
// la expiración sin esperar treinta días.
type RuleSubject struct {
	// Fingerprint es la huella del step tal como se acaba de componer, o la
	// cadena vacía si no se pudo componer o si nadie la pidió.
	Fingerprint string

	// Last es el último registro de la clave de posición del step (spec 11).
	Last state.StepRecord

	// Now es el instante de la ejecución en curso.
	Now time.Time
}

// Rule es una razón, DECLARADA POR EL PIPELINECODE, para desconfiar de lo
// guardado.
//
// La respuesta es un booleano y no un objeto con motivo, y es deliberado: el
// motivo es del step, no de la regla, porque lo que se emite es «este step se
// re-ejecuta porque X» y X es la primera regla que dijo que sí. Quien traduce
// `Kind()` a una frase es el handler, que es quien habla con el usuario.
//
// Es una Specification de verdad, no una que lo finge: cada regla es una
// especificación sobre el par (huella, registro) y el conjunto se combina con un
// OR — y la prueba de que lo es está en que **un conjunto con una sola regla se
// comporta exactamente como esa regla**. Es lo que `Policy` (spec 05) aparentaba
// sin serlo.
type Rule interface {
	// Kind es la clave con la que la regla se declara.
	Kind() RuleKind

	// Category obliga a que toda regla diga si afirma incorrección o sólo
	// vigencia. No es decorativa: de ella sale la advertencia de §5.4.
	Category() RuleCategory

	// IsSatisfiedBy responde «¿esta regla exige volver a ejecutar el step?».
	IsSatisfiedBy(subject RuleSubject) bool

	// Canonical es lo que la regla aporta a la IDENTIDAD de un step: su clave y
	// todo lo que la parametriza (spec 17 §5.1, spec 27 §5.2).
	//
	// Está en la interfaz por la misma razón que `Category()`: obliga a que la
	// tercera regla que alguien añada DECIDA qué aporta a la identidad, en vez de
	// aparecer como un caso más de un `switch` en otro paquete y contribuir sólo
	// su nombre. Una regla cuya parametrización no entrara en el hash sería una
	// que se puede cambiar sin que el step se re-ejecute — exactamente el defecto
	// que este catálogo lleva quince specs cerrando.
	//
	// Es la DECLARACIÓN, nunca el resultado de evaluarla: `max_age: 720h` entra;
	// «este registro caducó» no.
	Canonical() string
}

// ── state_changed ───────────────────────────────────────────────────────────

// StateChangedRule es la regla de INVALIDACIÓN: la huella del step difiere de la
// que quedó registrada en su último registro.
//
// Lo que declara —sus FUENTES— no participa de la evaluación: participa de la
// COMPOSICIÓN DEL MATERIAL. La regla siempre compara una huella con otra; lo que
// las fuentes deciden es qué entra en esa huella (`step.NewCacheMaterial`). Por
// eso `IsSatisfiedBy` es idéntica para `[pipeline]` y para `[pipeline, project]`
// y aun así los dos steps deciden distinto: el material del primero no lleva el
// código del proyecto, así que cambiarlo no mueve su huella.
//
// La guarda de la huella vacía viaja con la regla, y tiene que viajar con ella:
// `state.StepRecord.Revives` devuelve `false` ante una huella vacía **de
// cualquiera de los dos lados**, que es lo que conserva por construcción la
// semántica «sin evidencia ⇒ ejecutar» de la spec 05 §5.1. Un registro que no
// dice de qué contenido es no puede afirmar que ese contenido esté al día.
type StateChangedRule struct {
	watchesProject bool
}

var _ Rule = StateChangedRule{}

// NewDefaultStateChangedRule es la FORMA CORTA, `- state_changed` a secas, y
// equivale a `[pipeline, project]`.
//
// Que la forma corta incluya el proyecto, y no al revés, es el default seguro
// del catálogo: si significara «sólo mi declaración», un `01-test` escrito así
// DEJARÍA DE RE-EJECUTARSE ANTE UN CAMBIO DE CÓDIGO, que es la peor omisión que
// este motor puede cometer. Al revés el coste es una re-ejecución de más.
func NewDefaultStateChangedRule() StateChangedRule {
	return StateChangedRule{watchesProject: true}
}

// NewStateChangedRule es la forma larga, con las fuentes declaradas.
//
// El vocabulario es cerrado y `pipeline` es obligatorio. Un
// `state_changed: [project]` no es una configuración exótica: es un step
// declarando que su propio `commands.yaml` no le importa, que no configura nada
// —describe un bug— y por eso es un error de pipelinecode.
//
// La lista vacía es un error por lo mismo, y no la forma corta escrita raro:
// `state_changed: []` dice explícitamente «no vigilo nada», que es justo lo que
// `pipeline` no permite omitir.
func NewStateChangedRule(sources []string) (StateChangedRule, error) {
	if len(sources) == 0 {
		return StateChangedRule{}, fmt.Errorf(
			"'%s' sin fuentes no vigila nada: omite la lista para vigilarlo todo, "+
				"o declara al menos '%s'", RuleKindStateChanged, StateSourcePipeline)
	}

	declared := make(map[string]bool, len(sources))
	for _, source := range sources {
		switch source {
		case StateSourcePipeline, StateSourceProject:
		default:
			return StateChangedRule{}, fmt.Errorf(
				"'%s' no es una fuente de '%s': se espera %q o %q",
				source, RuleKindStateChanged, StateSourcePipeline, StateSourceProject)
		}
		if declared[source] {
			return StateChangedRule{}, fmt.Errorf(
				"'%s' declara la fuente '%s' dos veces", RuleKindStateChanged, source)
		}
		declared[source] = true
	}

	if !declared[StateSourcePipeline] {
		return StateChangedRule{}, fmt.Errorf(
			"'%s' no puede omitir '%s': un step que declara que su propia declaración "+
				"no le importa no está configurando nada",
			RuleKindStateChanged, StateSourcePipeline)
	}
	return StateChangedRule{watchesProject: declared[StateSourceProject]}, nil
}

func (r StateChangedRule) Kind() RuleKind         { return RuleKindStateChanged }
func (r StateChangedRule) Category() RuleCategory { return CategoryInvalidation }

// WatchesProject dice si el código del proyecto entra en la huella del step. Lo
// consulta el compositor del material, no el combinador.
func (r StateChangedRule) WatchesProject() bool { return r.watchesProject }

func (r StateChangedRule) IsSatisfiedBy(subject RuleSubject) bool {
	return !subject.Last.Revives(subject.Fingerprint)
}

// Canonical aporta la clave y las FUENTES declaradas, no la forma en que se
// escribieron: `- state_changed` a secas y `state_changed: [pipeline, project]`
// significan lo mismo y producen la misma cadena. La identidad es del
// significado; el azúcar sintáctico no la mueve.
func (r StateChangedRule) Canonical() string {
	sources := StateSourcePipeline
	if r.watchesProject {
		sources += "," + StateSourceProject
	}
	return string(RuleKindStateChanged) + ruleFieldSep + strconv.Quote(sources)
}

// ── max_age ─────────────────────────────────────────────────────────────────

// MaxAgeRule es la regla de EXPIRACIÓN: pasó más tiempo del declarado desde que
// se escribió el último registro.
//
// Sustituye al TTL global de 30 días, que era una política DEL MOTOR sobre
// despliegues ajenos. Quien sabe cada cuánto conviene revisar un despliegue es
// quien lo escribió, así que un step sin `max_age` **no caduca**.
//
// La edad se mide contra `produced_by.at` del último registro, que es
// exactamente contra lo que ya se medía: la spec 11 movió la expiración de
// sujeto —de la entrada de índice al registro— y esta spec sólo le cambia el
// dueño.
type MaxAgeRule struct {
	maxAge time.Duration
}

var _ Rule = MaxAgeRule{}

// NewMaxAgeRule toma la duración con la sintaxis de `time.ParseDuration`.
//
// **No hay unidad de días**: `30d` es un error y se escribe `720h`. Se conserva
// la gramática del lenguaje en vez de inventar una para no tener dos gramáticas
// de duración en el mismo binario — y el mensaje lo dice, porque `30d` es lo
// primero que alguien escribe.
//
// Una duración no positiva se rechaza: `max_age: 0s` significaría «caducado
// siempre», que ya se escribe sin declarar reglas, y tenerlo escrito de dos
// formas distintas es la clase de ambigüedad que este catálogo retira.
func NewMaxAgeRule(text string) (MaxAgeRule, error) {
	maxAge, err := time.ParseDuration(text)
	if err != nil {
		return MaxAgeRule{}, fmt.Errorf(
			"'%s: %s' no es una duración: se espera la sintaxis de Go ('30m', '6h', '720h'); "+
				"no hay unidad de días", RuleKindMaxAge, text)
	}
	if maxAge <= 0 {
		return MaxAgeRule{}, fmt.Errorf(
			"'%s: %s' no es una duración positiva: un registro que nace caducado es un step sin reglas",
			RuleKindMaxAge, text)
	}
	return MaxAgeRule{maxAge: maxAge}, nil
}

func (r MaxAgeRule) Kind() RuleKind         { return RuleKindMaxAge }
func (r MaxAgeRule) Category() RuleCategory { return CategoryExpiration }

// MaxAge es la duración declarada.
func (r MaxAgeRule) MaxAge() time.Duration { return r.maxAge }

// IsSatisfiedBy: el borde es EXCLUSIVO — el registro caduca CUANDO se alcanza su
// instante, no después. Es la misma frontera que comparaba la regla de tiempo
// antes de la spec 10, y la misma que comparaba el TTL global entre la 11 y la
// 15.
func (r MaxAgeRule) IsSatisfiedBy(subject RuleSubject) bool {
	return !subject.Now.Before(subject.Last.ProducedBy().At.Add(r.maxAge))
}

// Canonical aporta la clave y la duración NORMALIZADA por `time.Duration`, no el
// texto que se escribió: `max_age: 60m` y `max_age: 1h` declaran lo mismo y
// producen la misma cadena. Reescribir la unidad no es un cambio de intención, y
// por tanto no debe re-ejecutar nada.
func (r MaxAgeRule) Canonical() string {
	return string(RuleKindMaxAge) + ruleFieldSep + strconv.Quote(r.maxAge.String())
}
