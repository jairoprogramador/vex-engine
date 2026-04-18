package pipeline

import "errors"

type Environment struct {
	name  string
	value string
}

func NewEnvironment(value, name string) (Environment, error) {
	if value == "" {
		return Environment{}, errors.New("el valor del ambiente no debe estar vacío")
	}
	if name == "" {
		return Environment{}, errors.New("el nombre del ambiente no debe estar vacío")
	}
	return Environment{value: value, name: name}, nil
}

func (e Environment) Value() string {
	return e.value
}

func (e Environment) Name() string {
	return e.name
}

func (e Environment) Equals(other Environment) bool {
	return e.value == other.value
}
