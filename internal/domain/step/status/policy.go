package status

import (
	"errors"
	"fmt"
	"strings"
)

// ReasonNoRules es el motivo con el que una policy sin comprobaciones manda
// ejecutar. Es un motivo, no un error: el step corre y la línea de log dice por
// qué.
const ReasonNoRules = "no hay comprobaciones definidas para este paso"

// ReasonAllRulesPassed es el motivo del skip: todas las comprobaciones dijeron
// que nada cambió.
const ReasonAllRulesPassed = "all rules passed"

// ReasonUndetermined encabeza los motivos de las comprobaciones que no pudieron
// averiguar si algo cambió. Es lo que hace distinguible «no pude leer el estado
// anterior» de «el código cambió», que hasta la spec 09 producían exactamente la
// misma decisión con la misma forma de razón.
const ReasonUndetermined = "no se pudo determinar si cambió"

type Policy struct {
	name  string
	rules []Rule
}

var _ Rule = (*Policy)(nil)

func NewPolicy(name string, rules ...Rule) *Policy {
	if len(rules) == 0 {
		rules = make([]Rule, 0)
	}
	return &Policy{name: name, rules: rules}
}

func (p *Policy) Name() string {
	return p.name
}

// Evaluate agrupa las respuestas de las reglas POR ESTADO y devuelve, junto a la
// decisión, todo lo que las reglas observaron. La observación no se escribe aquí:
// viaja hasta el camino de éxito del step (spec 09 §5.2).
func (p *Policy) Evaluate(ctx RuleContext) (Decision, []Evidence, error) {
	// Cero reglas NO es «nada cambió»: es «no hay evidencia» (spec 05 §5.1).
	// Concluir que el step está al día a partir de un conjunto vacío de
	// comprobaciones invierte la implicación, y es lo que hacía que un step con
	// nombre desconocido no se ejecutara jamás, en silencio y con exit code 0.
	//
	// Ante la duda, ejecutar: ejecutar de más nunca produce un despliegue que no
	// ocurrió; saltar de menos sí. La guarda vive aquí —y no solo en el
	// PolicyBuilder— para que el defecto no reaparezca por otra vía de
	// construcción.
	//
	// Y es `Run`, NO `Undetermined` (spec 09 §5.3): «no hay nada que averiguar»
	// es una respuesta determinada. Meterla en el tercer estado la haría
	// indistinguible de un repositorio caído, y además es la respuesta que la
	// spec 15 hereda para una fase sin `checks`.
	if len(p.rules) == 0 {
		return DecisionRun(ReasonNoRules), nil, nil
	}

	var runReasons []string
	var undeterminedReasons []string
	var evidences []Evidence
	var errs []error

	for _, rule := range p.rules {
		decision, ruleEvidences, err := rule.Evaluate(ctx)
		evidences = append(evidences, ruleEvidences...)
		if err != nil {
			errs = append(errs, fmt.Errorf("[%s] %w", rule.Name(), err))
		}

		switch {
		case decision.IsUndetermined():
			undeterminedReasons = append(undeterminedReasons,
				fmt.Sprintf("[%s] %s", rule.Name(), decision.Reason()))
		case decision.ShouldRun():
			runReasons = append(runReasons, decision.Reason())
		}
	}

	// Un `Run` de verdad gana al `Undetermined`: si alguna comprobación SABE que
	// algo cambió, el motivo del step es ese cambio. Los indeterminados se
	// arrastran igualmente al final del motivo, porque siguen siendo un caché
	// roto que el usuario tiene que poder ver.
	if len(runReasons) > 0 {
		reason := strings.Join(runReasons, "; ")
		if len(undeterminedReasons) > 0 {
			reason += "; " + ReasonUndetermined + ": " + strings.Join(undeterminedReasons, "; ")
		}
		return DecisionRun(reason), evidences, errors.Join(errs...)
	}

	// Ninguna comprobación afirmó un cambio, pero alguna no pudo averiguarlo: se
	// ejecuta —fail-open— y se dice con esas palabras.
	if len(undeterminedReasons) > 0 {
		return DecisionUndetermined(ReasonUndetermined + ": " + strings.Join(undeterminedReasons, "; ")),
			evidences,
			errors.Join(errs...)
	}

	return DecisionSkip(ReasonAllRulesPassed), evidences, nil
}

func (p *Policy) AddRule(rule Rule) {
	p.rules = append(p.rules, rule)
}
