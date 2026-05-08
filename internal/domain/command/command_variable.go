package command

import "errors"

type CommandVariable struct {
	name     string
	value    string
	isShared bool
}

func NewCommandVariable(name, value string, isShared bool) (CommandVariable, error) {
	if name == "" {
		return CommandVariable{}, errors.New("el nombre de la variable generada no puede estar vacío")
	}
	if value == "" {
		return CommandVariable{}, errors.New("el valor de la variable generada no puede estar vacío")
	}

	return CommandVariable{
		isShared: isShared,
		name:     name,
		value:    value,
	}, nil
}

func (ve CommandVariable) IsShared() bool {
	return ve.isShared
}

func (ve CommandVariable) Name() string {
	return ve.name
}

func (ve CommandVariable) Value() string {
	return ve.value
}
