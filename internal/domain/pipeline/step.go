package pipeline

import (
	"errors"
	"strings"
)

type Step struct {
	name      StepName
	commands  []Command
	variables []Variable
}

type cmdKey struct{ name, cmd, workdir string }

func NewStep(
	name StepName,
	commands []Command,
	variables []Variable) (*Step, error) {

	if len(commands) == 0 {
		return nil, errors.New("un paso debe tener al menos un comando")
	}

	seen := make(map[cmdKey]bool)
	for _, c := range commands {
		key := cmdKey{
			name:    strings.ToUpper(strings.ReplaceAll(c.Name(), " ", "")),
			cmd:     strings.ReplaceAll(c.Cmd(), " ", ""),
			workdir: strings.ToUpper(strings.ReplaceAll(c.Workdir(), " ", "")),
		}
		if seen[key] {
			return nil, errors.New("comando duplicado dentro del mismo paso: " + c.Name())
		}
		seen[key] = true
	}

	varNames := make(map[string]bool)
	for _, v := range variables {
		vname := strings.ReplaceAll(v.Name(), " ", "")
		if varNames[vname] {
			return nil, errors.New("variable duplicada: " + vname)
		}
		varNames[vname] = true
	}

	return &Step{
		name:      name,
		commands:  commands,
		variables: variables,
	}, nil
}

func (s *Step) Name() StepName {
	return s.name
}

func (s *Step) Commands() []Command {
	return s.commands
}

func (s *Step) Variables() []Variable {
	return s.variables
}
