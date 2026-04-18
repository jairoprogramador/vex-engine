package pipeline

import (
	"errors"
)

type Command struct {
	name          string
	description   string
	cmd           string
	workdir       string
	templateFiles []string
	outputs       []Output
}

type CommandOption func(*Command)

func NewCommand(name, cmd string, opts ...CommandOption) (Command, error) {
	if name == "" {
		return Command{}, errors.New("el nombre del comando debe estar vacío")
	}
	if cmd == "" {
		return Command{}, errors.New("el comando debe estar vacío")
	}

	cmdDef := &Command{
		name: name,
		cmd:  cmd,
	}

	for _, opt := range opts {
		opt(cmdDef)
	}

	if len(cmdDef.templateFiles) > 0 {
		templateFilesMap := make(map[string]struct{})
		for _, file := range cmdDef.templateFiles {
			if _, exists := templateFilesMap[file]; exists {
				return Command{}, errors.New("template duplicado")
			}
			templateFilesMap[file] = struct{}{}
		}
	}

	if len(cmdDef.outputs) > 0 {
		outputNames := make(map[string]struct{})
		for _, output := range cmdDef.outputs {
			if _, exists := outputNames[output.Name()]; exists {
				return Command{}, errors.New("output duplicado")
			}
			outputNames[output.Name()] = struct{}{}
		}
	}

	return *cmdDef, nil
}

func WithWorkdir(workdir string) CommandOption {
	return func(c *Command) {
		c.workdir = workdir
	}
}

func WithDescription(description string) CommandOption {
	return func(c *Command) {
		c.description = description
	}
}

func WithTemplateFiles(files []string) CommandOption {
	return func(c *Command) {
		c.templateFiles = files
	}
}

func WithOutputs(outputs []Output) CommandOption {
	return func(c *Command) {
		c.outputs = outputs
	}
}

func (cd Command) Name() string {
	return cd.name
}

func (cd Command) Description() string {
	return cd.description
}

func (cd Command) Cmd() string {
	return cd.cmd
}

func (cd Command) Workdir() string {
	return cd.workdir
}

func (cd Command) TemplateFiles() []string {
	filesCopy := make([]string, len(cd.templateFiles))
	copy(filesCopy, cd.templateFiles)
	return filesCopy
}

func (cd Command) Outputs() []Output {
	outputsCopy := make([]Output, len(cd.outputs))
	copy(outputsCopy, cd.outputs)
	return outputsCopy
}
