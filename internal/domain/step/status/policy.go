package status

import "fmt"

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
	for _, rule := range p.rules {
		result, err := rule.Evaluate(ctx)
		if err != nil {
			return DecisionRun(fmt.Sprintf("rule %q failed: %s", rule.Name(), err)), err
		}
		if result.ShouldRun() {
			return result, nil
		}
	}
	return DecisionSkip("all rules passed"), nil
}

func (p *Policy) AddRule(rule Rule) {
	p.rules = append(p.rules, rule)
}
