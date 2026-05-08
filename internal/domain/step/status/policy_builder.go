package status

import "errors"

var PolicyTestRulesNames = []string{
	InstPipelineRuleName,
	VariablesRuleName,
	CodeProjectRuleName,
	TimeRuleName,
}

var PolicySupplyRulesNames = []string{
	InstPipelineRuleName,
	VariablesRuleName,
}

var PolicyPackageRulesNames = []string{
	InstPipelineRuleName,
	VariablesRuleName,
	CodeProjectRuleName,
}

var PolicyDeployRulesNames = []string{
	InstPipelineRuleName,
	VariablesRuleName,
	CodeProjectRuleName,
}

const (
	StepTest    = "test"
	StepSupply  = "supply"
	StepPackage = "package"
	StepDeploy  = "deploy"
)

type PolicyBuilder struct {
	registry *RuleRegistry
	errs     []error
}

func NewPolicyBuilder(registry *RuleRegistry) *PolicyBuilder {
	return &PolicyBuilder{
		registry: registry,
	}
}

func (b *PolicyBuilder) Build(stepName string) (*Policy, error) {
	if len(b.errs) > 0 {
		return nil, errors.Join(b.errs...)
	}

	rules := make([]Rule, 0)
	for _, ruleName := range b.getArrayRulesNames(stepName) {
		rule, err := b.registry.Get(ruleName)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}

	return NewPolicy(stepName, rules...), nil
}

func (b *PolicyBuilder) getArrayRulesNames(stepName string) []string {
	switch stepName {
	case StepTest:
		return PolicyTestRulesNames
	case StepSupply:
		return PolicySupplyRulesNames
	case StepPackage:
		return PolicyPackageRulesNames
	case StepDeploy:
		return PolicyDeployRulesNames
	}
	return make([]string, 0)
}
