package step

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// VariableSource es lo que el CONSUMIDOR declara sobre de dónde sale el valor de
// una variable que no define como literal (spec 14 §5.2).
//
// Hasta esta spec no había dónde declararlo: `PipelineVariableDTO` era
// exactamente `{name, description, value}`, así que una variable producida por un
// step aparecía en el mapa acumulado sin que nadie hubiera dicho de dónde venía y
// el consumidor la nombraba y ya. El acoplamiento era por NOMBRE, en un espacio
// plano, resuelto por orden de ejecución — y por tanto no conocible antes de
// ejecutar, que es justo lo que la identidad de una ejecución necesita saber.
//
// El vocabulario es CERRADO, por la misma razón que el de los ámbitos: un origen
// inventado no tendría resolutor, y además entra en un hash permanente. Crece
// añadiendo un valor y su resolutor (OCP, §5.2'), no editando un `switch` que
// mezcle los tres.
type VariableSource string

const (
	// SourceLiteral es la ausencia de `resolve:`: el valor lo escribió una
	// persona en el pipelinecode. Es el valor CERO del tipo, y eso es deliberado:
	// un `variables/<ambiente>/<paso>.yaml` de siempre sigue significando lo
	// mismo sin tocar una línea.
	SourceLiteral VariableSource = ""

	// SourceStepOutput es «lo produce el `outputs` de un step de este pipeline».
	// Exige `from` (qué step) y `key` (qué output).
	SourceStepOutput VariableSource = "step-output"

	// SourceState es «lo dejó una ejecución anterior en el registro de este step,
	// bajo otro ámbito» (spec 11). Exige `scope` y `key`.
	SourceState VariableSource = "state"
)

// ErrDeclarationNameEmpty es el invariante que la declaración comparte con
// `command.Variable`: el `name:` sale del pipelinecode, así que vacío es un
// pipelinecode mal escrito y no un dato.
var ErrDeclarationNameEmpty = errors.New("la variable declarada no tiene 'name'")

// NewVariableSource traduce lo declarado en `resolve:`. La cadena vacía NO es un
// error: es el literal de siempre.
func NewVariableSource(text string) (VariableSource, error) {
	switch VariableSource(text) {
	case SourceLiteral, SourceStepOutput, SourceState:
		return VariableSource(text), nil
	default:
		return SourceLiteral, fmt.Errorf(
			"'resolve: %s' no es un origen: se espera %q o %q",
			text, SourceStepOutput, SourceState)
	}
}

// VariableDeclaration es lo que el pipelinecode DICE de una variable, y es el
// material de identidad (spec 14 §5.1').
//
// Es la mitad del concepto que hasta ahora era uno solo: `VariableDeclaration` es
// la declaración —estable, conocible antes de ejecutar, hasheable— y
// `command.Variable` es el RESULTADO de resolverla. Con las dos separadas, el
// mapa acumulado deja de ser el modelo y pasa a ser lo que siempre fue de facto:
// una caché de resolución.
//
// Vive en `domain/step` y no en `domain/pipeline` como decía §6, y no por
// descuido: la regla de dependencias es `pipeline → step`, así que una
// declaración en `pipeline` sería inalcanzable desde el handler que la resuelve.
// Aquí además reutiliza `Scope`, que es el mismo vocabulario cerrado que declara
// el `config.yaml` del step — tenerlo escrito dos veces era exactamente el modo
// de fallo que la spec 13 retiró.
type VariableDeclaration struct {
	name   string
	source VariableSource

	// value es el literal, y sólo lo tiene `SourceLiteral`. Vacío es un valor
	// legítimo desde la spec 03 §5.1: «declarada y vacía» y «no declarada» son
	// estados distintos y producen material de identidad distinto.
	value string

	// from es el step que produce el valor, y sólo lo tiene `SourceStepOutput`.
	// Es el dato que vuelve EXPLÍCITO el grafo de dependencias entre steps, y con
	// él las dos validaciones de §5.4 que hoy son imposibles.
	from string

	// key es el nombre en la fuente: el `outputs[].name` del step productor, o la
	// clave dentro del registro de estado. Que pueda diferir de `name` es lo que
	// permite que el consumidor la llame como quiera sin que el productor sepa
	// quién lo lee.
	key string

	// scope es el ámbito del registro, y sólo lo tiene `SourceState`.
	scope Scope
}

// NewLiteralDeclaration es el caso (a) de §5.2: la variable de siempre.
func NewLiteralDeclaration(name, value string) (VariableDeclaration, error) {
	if name == "" {
		return VariableDeclaration{}, ErrDeclarationNameEmpty
	}
	return VariableDeclaration{name: name, source: SourceLiteral, value: value}, nil
}

// NewStepOutputDeclaration es el caso (b): lo produce el `outputs` de un step.
//
// `from` y `key` ausentes son un ERROR, no un default (§5.2). Adivinar cuál es
// el step productor es la alternativa D —inferir— que se descartó: dos steps que
// declaran el mismo nombre de output son indistinguibles, y la inferencia
// quedaría congelada dentro de un identificador permanente.
func NewStepOutputDeclaration(name, from, key string) (VariableDeclaration, error) {
	if name == "" {
		return VariableDeclaration{}, ErrDeclarationNameEmpty
	}
	if from == "" {
		return VariableDeclaration{}, fmt.Errorf(
			"la variable '%s' declara 'resolve: %s' y no dice 'from': falta el step que la produce",
			name, SourceStepOutput)
	}
	if key == "" {
		return VariableDeclaration{}, fmt.Errorf(
			"la variable '%s' declara 'resolve: %s' y no dice 'key': falta el output que la produce",
			name, SourceStepOutput)
	}
	return VariableDeclaration{name: name, source: SourceStepOutput, from: from, key: key}, nil
}

// NewStateDeclaration es el caso (c): lo dejó una ejecución anterior en el
// registro de este step bajo el ámbito declarado.
//
// El ámbito es el MISMO vocabulario cerrado del `config.yaml` (spec 13), y la
// clave que se lee es la de posición de este step bajo ese ámbito. Es la mitad
// LECTORA de la asimetría de la spec 13 §5.4 —se leen los dos ámbitos, se
// escribe en uno— convertida en algo que se declara en vez de ocurrir sola.
func NewStateDeclaration(name, scope, key string) (VariableDeclaration, error) {
	if name == "" {
		return VariableDeclaration{}, ErrDeclarationNameEmpty
	}
	declaredScope, err := NewScope(scope)
	if err != nil {
		return VariableDeclaration{}, fmt.Errorf(
			"la variable '%s' declara 'resolve: %s' y %w", name, SourceState, err)
	}
	if key == "" {
		return VariableDeclaration{}, fmt.Errorf(
			"la variable '%s' declara 'resolve: %s' y no dice 'key': falta qué se lee del registro",
			name, SourceState)
	}
	return VariableDeclaration{name: name, source: SourceState, scope: declaredScope, key: key}, nil
}

func (d VariableDeclaration) Name() string           { return d.name }
func (d VariableDeclaration) Source() VariableSource { return d.source }
func (d VariableDeclaration) Value() string          { return d.value }
func (d VariableDeclaration) From() string           { return d.from }
func (d VariableDeclaration) Key() string            { return d.key }
func (d VariableDeclaration) Scope() Scope           { return d.scope }

// IsLiteral dice si el valor lo escribió una persona en el pipelinecode.
func (d VariableDeclaration) IsLiteral() bool { return d.source == SourceLiteral }

// declarationFieldSep separa los campos dentro de la forma canónica. Es el mismo
// separador que usan las reglas de huella para sus entradas: un byte que no
// aparece en un YAML escrito por una persona, así que dos declaraciones distintas
// no pueden producir la misma cadena por concatenación.
const declarationFieldSep = "\x1e"

// Canonical es la forma canónica de la declaración: lo que ENTRA en la identidad
// (§5.3).
//
// Entran `name`, `resolve`, `from`, `key`, `scope` y el `value` literal cuando lo
// hay. NO entra el valor RESUELTO, y ésa es la regla que gobierna el resto del
// catálogo: en la identidad entra la declaración, nunca el valor —el valor
// resuelto se registra aparte, como hecho, en la spec 19—.
//
// Es discriminante por construcción, que es la razón por la que se descartó el
// marcador genérico `dynamic: true`: dos declaraciones que sólo difieren en su
// `from` producen cadenas distintas, mientras que un «esto se resuelve luego»
// produciría la misma para todas y una parte constante de un hash no aporta
// identidad.
//
// Se calcula UNA vez y se comparte: la misma estructura alimenta el material de
// la huella de hoy, `pipe-v1` (spec 27) y `content.parameters` (spec 17).
func (d VariableDeclaration) Canonical() string {
	switch d.source {
	case SourceStepOutput:
		return strings.Join([]string{
			string(SourceStepOutput),
			strconv.Quote(d.from),
			strconv.Quote(d.key),
		}, declarationFieldSep)
	case SourceState:
		return strings.Join([]string{
			string(SourceState),
			strconv.Quote(d.scope.String()),
			strconv.Quote(d.key),
		}, declarationFieldSep)
	default:
		return strconv.Quote(d.value)
	}
}

// SourceDescription nombra la FUENTE en los mensajes de error. Es la diferencia
// observable de §5.5: hasta esta spec una variable que no llegaba fallaba con
// «variable no existe», que no dice a quién reclamarle.
func (d VariableDeclaration) SourceDescription() string {
	switch d.source {
	case SourceStepOutput:
		return fmt.Sprintf("el output '%s' del step '%s'", d.key, d.from)
	case SourceState:
		return fmt.Sprintf("la clave '%s' del registro de ámbito '%s'", d.key, d.scope)
	default:
		return "el literal declarado"
	}
}
