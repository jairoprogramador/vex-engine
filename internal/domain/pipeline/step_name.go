package pipeline

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
)

var stepNameRegex = regexp.MustCompile(`^(\d+)-(.+)$`)

type StepName struct {
	order int
	name  string
}

// NewStepName parsea un nombre de directorio con formato NN-nombre.
// El formato %02d limita el orden a 99 steps por diseño.
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
