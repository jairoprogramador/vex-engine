package step

import (
	"fmt"
	"slices"
	"strings"
)

// RuleSet son las reglas que un step declara, combinadas con OR.
//
// # Por qué OR
//
// Si CUALQUIERA de las reglas declaradas se cumple, el step se re-ejecuta. Es la
// única combinación coherente con lo que las reglas afirman: cada una es una
// razón INDEPENDIENTE para desconfiar de lo guardado, y exigir que coincidan dos
// razones independientes equivaldría a ignorar la primera que aparezca.
//
// # La invariante que este tipo custodia
//
// **Un conjunto vacío no evalúa a «revivir».** El OR sobre un conjunto vacío es
// falso, así que la lectura literal diría «revivir» — y eso es exactamente el
// defecto que le da título a la spec 05: un step sin reglas se salta para
// siempre, corre una vez y nunca vuelve a correr.
//
// «No hay nada que comprobar» no es «esta configuración está al día». Un
// conjunto vacío de evidencia no concluye nada, y concluir a partir de él es la
// única forma en que este motor puede omitir un despliegue en silencio.
//
// La guarda vive AQUÍ y no en el llamador porque el llamador es donde se perdió
// la vez anterior.
type RuleSet struct {
	rules []Rule

	// stateChanged se resuelve al construir, no al preguntar. El combinador
	// trata las dos reglas por igual —eso es lo que hace que el OR no sea
	// mentira—; quien necesita distinguirlas es el COMPOSITOR DEL MATERIAL, que
	// hace otra pregunta: no «¿se cumple?» sino «¿qué entra en la huella?».
	stateChanged StateChangedRule
	watchesState bool
}

// EmptyRuleSet es el conjunto sin reglas: el step se ejecuta siempre y no
// escribe registro (§5.5). Es lo que lleva un `config.yaml` sin `rules`.
func EmptyRuleSet() RuleSet { return RuleSet{} }

// NewRuleSet compone el conjunto en el ORDEN DECLARADO, que es el orden en que
// se evalúa y por tanto el que decide qué motivo se emite: el de la primera
// regla que dijo que sí.
//
// Dos reglas de la misma clave son un error. Con un OR, `max_age: 1h` y
// `max_age: 720h` en el mismo `rules` no componen nada —la primera gana siempre—
// así que declararlas es contradecirse, y el motor no puede saber cuál se quiso.
func NewRuleSet(rules ...Rule) (RuleSet, error) {
	set := RuleSet{rules: slices.Clone(rules)}

	declared := make(map[RuleKind]bool, len(rules))
	for _, rule := range rules {
		if declared[rule.Kind()] {
			return RuleSet{}, fmt.Errorf("'rules' declara '%s' dos veces", rule.Kind())
		}
		declared[rule.Kind()] = true

		if changed, ok := rule.(StateChangedRule); ok {
			set.stateChanged, set.watchesState = changed, true
		}
	}
	return set, nil
}

// IsEmpty dice que no hay nada que comprobar. Aguas arriba significa las dos
// mitades de §5.5: ejecutar siempre, y no escribir registro.
func (s RuleSet) IsEmpty() bool { return len(s.rules) == 0 }

// Rules son las reglas declaradas, en su orden.
func (s RuleSet) Rules() []Rule { return slices.Clone(s.rules) }

// RequiresRun es el OR, y devuelve QUIÉN lo cerró para que el step pueda decir
// por qué se re-ejecuta.
//
// El conjunto vacío devuelve `(RuleKindNone, true)`: ejecutar, sin regla que
// nombrar. Es la invariante de arriba, escrita en el sitio donde no se puede
// olvidar.
func (s RuleSet) RequiresRun(subject RuleSubject) (RuleKind, bool) {
	if s.IsEmpty() {
		return RuleKindNone, true
	}
	for _, rule := range s.rules {
		if rule.IsSatisfiedBy(subject) {
			return rule.Kind(), true
		}
	}
	return RuleKindNone, false
}

// StateChanged devuelve la regla de invalidación declarada, si la hay.
//
// La consulta el compositor del material —para saber si el código del proyecto
// entra en la huella— y el propio handler, para saber si hay que componer una
// huella siquiera. Un step que no declara `state_changed` no compara huellas, así
// que calcularle una sería trabajo cuyo resultado nadie lee.
func (s RuleSet) StateChanged() (StateChangedRule, bool) {
	return s.stateChanged, s.watchesState
}

// ruleSetSep separa una regla de la siguiente en la forma canónica del
// conjunto. Es DISTINTO de `ruleFieldSep` a propósito: con el mismo separador en
// los dos niveles, un conjunto de dos reglas y una sola regla con dos campos
// podrían producir la misma cadena, y la regla dejaría de ser inyectiva.
const ruleSetSep = "\x1f"

// Canonical es lo que el conjunto aporta a la IDENTIDAD de un step: las reglas
// declaradas, cada una con su parametrización, EN SU ORDEN.
//
// El orden entra porque significa algo: es el orden de evaluación, y por tanto
// el que decide qué motivo se emite cuando dos reglas se cumplen a la vez.
// Reordenar `rules` cambia lo que el usuario verá, así que cambia el contenido.
//
// El conjunto vacío produce la cadena vacía, que es la forma canónica de «este
// step no declara nada que invalide su trabajo» — un hecho, distinguible de «no
// hay `config.yaml`» porque ese caso además no declara ámbito.
func (s RuleSet) Canonical() string {
	if s.IsEmpty() {
		return ""
	}
	canonical := make([]string, 0, len(s.rules))
	for _, rule := range s.rules {
		canonical = append(canonical, rule.Canonical())
	}
	return strings.Join(canonical, ruleSetSep)
}

// ExpiresWithoutInvalidating es la configuración válida y peligrosa de §5.4:
// reglas de expiración y ninguna de invalidación.
//
// Un step así revive un resultado obsoleto durante toda su ventana de vigencia
// aunque su huella haya cambiado de forma evidente. **No es un error y no se
// prohíbe** —nada es implícito, todo se declara— pero se avisa, para que sea una
// elección consciente y no un olvido.
//
// Se pregunta por CATEGORÍA y no por `RuleKindMaxAge` a propósito: la tercera
// regla de expiración que alguien añada entra aquí sola.
func (s RuleSet) ExpiresWithoutInvalidating() bool {
	expires := false
	for _, rule := range s.rules {
		switch rule.Category() {
		case CategoryInvalidation:
			return false
		case CategoryExpiration:
			expires = true
		}
	}
	return expires
}
