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

func (p *Policy) Evaluate(ctx RuleContext) (Decision, error) {
	// Cero reglas NO es «nada cambió»: es «no hay evidencia» (spec 05 §5.1).
	// Concluir que el step está al día a partir de un conjunto vacío de
	// comprobaciones invierte la implicación, y es lo que hacía que un step con
	// nombre desconocido no se ejecutara jamás, en silencio y con exit code 0.
	//
	// Ante la duda, ejecutar: ejecutar de más nunca produce un despliegue que no
	// ocurrió; saltar de menos sí. La guarda vive aquí —y no solo en el
	// PolicyBuilder— para que el defecto no reaparezca por otra vía de
	// construcción.
	if len(p.rules) == 0 {
		return DecisionRun(ReasonNoRules), nil
	}

	var runReasons []string
	var errs []error

	for _, rule := range p.rules {
		result, err := rule.Evaluate(ctx)
		if err != nil {
			runReasons = append(runReasons, fmt.Sprintf("[%s] error: %s", rule.Name(), err))
			errs = append(errs, err)
			continue
		}
		if result.ShouldRun() {
			runReasons = append(runReasons, fmt.Sprintf("%s", result.Reason()))
		}
	}

	if len(runReasons) > 0 {
		return DecisionRun(strings.Join(runReasons, "; ")), errors.Join(errs...)
	}
	return DecisionSkip(ReasonAllRulesPassed), nil
}

func (p *Policy) AddRule(rule Rule) {
	p.rules = append(p.rules, rule)
}
