package command

import (
	"errors"
)

type Command struct {
	name          string
	cmd           string
	workdir       CommandWorkdir
	templatePaths []CommandTemplatePath
	outputs       []CommandOutput
}

type CommandOption func(*Command)

func NewCommand(name, cmd string, opts ...CommandOption) (Command, error) {
	if name == "" {
		return Command{}, errors.New("el nombre de comando no puede estar vacío")
	}
	if cmd == "" {
		return Command{}, errors.New("el comando no puede estar vacío")
	}

	command := Command{
		name:    name,
		cmd:     cmd,
		workdir: NewCommandWorkdir(""),
	}

	for _, opt := range opts {
		opt(&command)
	}

	return command, nil
}

func WithWorkdir(workdir string) CommandOption {
	return func(c *Command) {
		c.workdir = NewCommandWorkdir(workdir)
	}
}

func WithOutputs(outputs []CommandOutput) CommandOption {
	return func(c *Command) {
		c.outputs = outputs
	}
}

func WithTemplateFiles(templateFiles []CommandTemplatePath) CommandOption {
	return func(c *Command) {
		c.templatePaths = templateFiles
	}
}
func (cd Command) Name() string {
	return cd.name
}

func (cd Command) Cmd() string {
	return cd.cmd
}

func (cd Command) Workdir() CommandWorkdir {
	return cd.workdir
}

func (cd Command) TemplatePaths() []CommandTemplatePath {
	templatePathsCopy := make([]CommandTemplatePath, len(cd.templatePaths))
	copy(templatePathsCopy, cd.templatePaths)
	return templatePathsCopy
}

func (cd Command) Outputs() []CommandOutput {
	outputsCopy := make([]CommandOutput, len(cd.outputs))
	copy(outputsCopy, cd.outputs)
	return outputsCopy
}
