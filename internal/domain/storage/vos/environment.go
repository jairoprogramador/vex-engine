package vos

import "errors"

type Environment struct {
	value string
}

func NewEnvironment(value string) (Environment, error) {
	if value == "" {
		return Environment{}, errors.New("environment value cannot be empty")
	}
	return Environment{value: value}, nil
}

func (e Environment) String() string {
	return e.value
}

func (e Environment) Equals(other Environment) bool {
	return e.value == other.value
}
