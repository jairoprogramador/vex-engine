package command

import (
	"errors"
	"fmt"
	"regexp"
)

type CommandOutput struct {
	name          string
	probe         string
	compiledProbe *regexp.Regexp
}

func NewCommandOutput(name, probe string) (CommandOutput, error) {
	if probe == "" {
		return CommandOutput{}, errors.New("la expresión de la sonda de comando no puede estar vacía")
	}
	re, err := regexp.Compile(probe)
	if err != nil {
		return CommandOutput{}, fmt.Errorf("probe '%s' tiene regex inválida: %w", probe, err)
	}
	return CommandOutput{name: name, probe: probe, compiledProbe: re}, nil
}

func (op CommandOutput) Name() string                  { return op.name }
func (op CommandOutput) Probe() string                 { return op.probe }
func (op CommandOutput) CompiledProbe() *regexp.Regexp { return op.compiledProbe }
