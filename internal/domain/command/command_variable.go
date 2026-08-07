package command

import "errors"

// CommandVariable es lo que un comando extrajo de su propio stdout.
//
// Perdió `isShared` con la spec 13 §5.6, por la misma razón que `Variable`: el
// ámbito lo declara el step, así que una variable no puede tener uno distinto
// del de su step.
type CommandVariable struct {
	name  string
	value string
}

func NewCommandVariable(name, value string) (CommandVariable, error) {
	if name == "" {
		return CommandVariable{}, errors.New("el nombre de la variable generada no puede estar vacío")
	}
	if value == "" {
		return CommandVariable{}, errors.New("el valor de la variable generada no puede estar vacío")
	}

	return CommandVariable{
		name:  name,
		value: value,
	}, nil
}

func (ve CommandVariable) Name() string {
	return ve.name
}

func (ve CommandVariable) Value() string {
	return ve.value
}
