package step

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	"gopkg.in/yaml.v3"
)

type pipelineCommandRepository struct{}

func NewPipelineCommandRepository() domStep.PipelineCommandRepository {
	return &pipelineCommandRepository{}
}

func (r *pipelineCommandRepository) Get(ctx *context.Context, pipelineLocalPath, step string) ([]command.Command, error) {
	return r.readCommandsFromFile(*ctx, filepath.Join(pipelineLocalPath, "steps", step, "commands.yaml"))
}

func (r *pipelineCommandRepository) readCommandsFromFile(_ context.Context, filePath string) ([]command.Command, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []command.Command{}, nil
		}
		return nil, err
	}

	var commandsDTO []PipelineCommandDTO
	if err := yaml.Unmarshal(data, &commandsDTO); err != nil {
		return nil, fmt.Errorf("parsear YAML de comandos '%s': %w", filePath, err)
	}

	commands := make([]command.Command, 0, len(commandsDTO))
	for _, cmdDTO := range commandsDTO {
		outputs := make([]command.CommandOutput, 0, len(cmdDTO.Outputs))
		for _, outDTO := range cmdDTO.Outputs {
			out, err := command.NewCommandOutput(outDTO.Name, outDTO.Probe)
			if err != nil {
				return nil, fmt.Errorf("output inválido en comando '%s': %w", cmdDTO.Name, err)
			}
			outputs = append(outputs, out)
		}

		templates := make([]command.CommandTemplatePath, 0, len(cmdDTO.TemplateFiles))
		for _, template := range cmdDTO.TemplateFiles {
			templates = append(templates, command.NewCommandTemplatePath(template))
		}

		cmd, err := command.NewCommand(
			cmdDTO.Name,
			cmdDTO.Cmd,
			command.WithWorkdir(cmdDTO.Workdir),
			command.WithTemplateFiles(templates),
			command.WithOutputs(outputs),
		)
		if err != nil {
			return nil, fmt.Errorf("comando inválido '%s': %w", cmdDTO.Name, err)
		}
		commands = append(commands, cmd)
	}
	return commands, nil
}
