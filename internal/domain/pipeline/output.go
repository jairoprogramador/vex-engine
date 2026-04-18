package pipeline

import "errors"

type Output struct {
	name        string
	description string
	probe       string // Regex
}

func NewOutput(name, description, probe string) (Output, error) {
	if name == "" {
		return Output{}, errors.New("el nombre del output no debe estar vacío")
	}
	if probe == "" {
		return Output{}, errors.New("el regex del output no debe estar vacío")
	}
	return Output{
		name:        name,
		description: description,
		probe:       probe,
	}, nil
}

func (o Output) Name() string {
	return o.name
}

func (o Output) Description() string {
	return o.description
}

func (o Output) Probe() string {
	return o.probe
}
