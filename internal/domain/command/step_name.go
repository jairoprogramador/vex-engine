package command

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

type StepNameValue string

const (
	StepTest    StepNameValue = "test"
	StepSupply  StepNameValue = "supply"
	StepPackage StepNameValue = "package"
	StepDeploy  StepNameValue = "deploy"
)

var stepNameRegex = regexp.MustCompile(`^(\d+)-(.+)$`)

type StepName struct {
	order int
	name  string
}

func NewStepName(dirName string) (StepName, error) {
	matches := stepNameRegex.FindStringSubmatch(dirName)
	if len(matches) != 3 {
		return StepName{}, fmt.Errorf("el nombre del directorio del paso '%s' no sigue el formato 'NN-nombre'", dirName)
	}

	order, err := strconv.Atoi(matches[1])
	if err != nil {
		return StepName{}, fmt.Errorf("no se pudo parsear el número de orden del paso '%s'", dirName)
	}

	name := matches[2]
	if name == "" {
		return StepName{}, errors.New("el nombre del paso no puede estar vacío")
	}

	return StepName{order: order, name: name}, nil
}

func (s StepName) Order() int {
	return s.order
}

func (s StepName) Name() string {
	return s.name
}

func (s StepName) FullName() string {
	return fmt.Sprintf("%02d-%s", s.order, s.name)
}
