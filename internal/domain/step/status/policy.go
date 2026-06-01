package status

import (
	"errors"
	"fmt"
	"strings"
)

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
	return DecisionSkip("all rules passed"), nil
}

func (p *Policy) AddRule(rule Rule) {
	p.rules = append(p.rules, rule)
}
