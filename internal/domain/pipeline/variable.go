package pipeline

import "errors"

type Variable struct {
	name  string
	value string
}

func NewVariable(name string, value string) (Variable, error) {
	if name == "" {
		return Variable{}, errors.New("el nombre de la variable no debe estar vacío")
	}
	if value == "" {
		return Variable{}, errors.New("el valor de la variable no debe estar vacío")
	}
	return Variable{name: name, value: value}, nil
}

func (v Variable) Name() string {
	return v.name
}

func (v Variable) Value() string {
	return v.value
}
