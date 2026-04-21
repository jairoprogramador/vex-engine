package vos

import "fmt"

type StepName string

const (
	StepTest    StepName = "test"
	StepSupply  StepName = "supply"
	StepPackage StepName = "package"
	StepDeploy  StepName = "deploy"
)

var knownSteps = map[StepName]struct{}{
	StepTest:    {},
	StepSupply:  {},
	StepPackage: {},
	StepDeploy:  {},
}

// NewStepName valida que el string corresponda a un paso conocido del sistema.
func NewStepName(s string) (StepName, error) {
	name := StepName(s)
	if _, ok := knownSteps[name]; !ok {
		return "", fmt.Errorf("paso desconocido: %q; valores válidos: test, supply, package, deploy", s)
	}
	return name, nil
}

func (s StepName) String() string {
	return string(s)
}
