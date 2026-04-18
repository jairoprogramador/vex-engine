package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	dom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/domain/pipeline/ports"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline/dto"
	"gopkg.in/yaml.v3"
)

// yamlFileReader implementa services.Reader leyendo archivos YAML del filesystem.
// Es la única capa que conoce la estructura de directorios del repositorio pipelinecode.
type yamlFileReader struct{}

func NewYamlPipelineReader() ports.PipelineReader {
	return &yamlFileReader{}
}

var _ ports.PipelineReader = (*yamlFileReader)(nil)

func (r *yamlFileReader) ReadEnvironments(ctx context.Context, pathBase string) ([]dom.Environment, error) {
	return r.readEnvironmentsFromFile(ctx, filepath.Join(pathBase, "environments.yaml"))
}

func (r *yamlFileReader) ReadStepNames(ctx context.Context, pathBase string) ([]dom.StepName, error) {
	return r.readStepNamesFromDir(ctx, filepath.Join(pathBase, "steps"))
}

func (r *yamlFileReader) ReadCommands(ctx context.Context, pathBase string, stepName dom.StepName) ([]dom.Command, error) {
	return r.readCommandsFromFile(ctx, filepath.Join(pathBase, "steps", stepName.FullName(), "commands.yaml"))
}

func (r *yamlFileReader) ReadVariables(ctx context.Context, pathBase string, env dom.Environment, stepName dom.StepName) ([]dom.Variable, error) {
	path := filepath.Join(pathBase, "variables", env.Value(), stepName.Name()+".yaml")
	return r.readVariablesFromFile(ctx, path)
}

func (r *yamlFileReader) readEnvironmentsFromFile(_ context.Context, sourcePath string) ([]dom.Environment, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []dom.Environment{}, nil
		}
		return nil, err
	}

	var dtos []dto.EnvironmentDTO
	if err := yaml.Unmarshal(data, &dtos); err != nil {
		return nil, fmt.Errorf("parsear YAML de entornos: %w", err)
	}

	environments := make([]dom.Environment, 0, len(dtos))
	for _, envDTO := range dtos {
		env, err := dom.NewEnvironment(envDTO.Value, envDTO.Name)
		if err != nil {
			return nil, fmt.Errorf("entorno inválido en el archivo de definición: %w", err)
		}
		environments = append(environments, env)
	}
	return environments, nil
}

func (r *yamlFileReader) readStepNamesFromDir(_ context.Context, stepsDir string) ([]dom.StepName, error) {
	files, err := os.ReadDir(stepsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []dom.StepName{}, nil
		}
		return nil, err
	}

	stepNames := make([]dom.StepName, 0)
	for _, file := range files {
		if file.IsDir() {
			stepName, err := dom.NewStepName(file.Name())
			if err == nil {
				stepNames = append(stepNames, stepName)
			}
		}
	}
	return stepNames, nil
}

func (r *yamlFileReader) readCommandsFromFile(_ context.Context, commandsFilePath string) ([]dom.Command, error) {
	data, err := os.ReadFile(commandsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []dom.Command{}, nil
		}
		return nil, err
	}

	var dtos []dto.CommandDTO
	if err := yaml.Unmarshal(data, &dtos); err != nil {
		return nil, fmt.Errorf("parsear YAML de comandos '%s': %w", commandsFilePath, err)
	}

	commands := make([]dom.Command, 0, len(dtos))
	for _, cmdDTO := range dtos {
		outputs := make([]dom.Output, 0, len(cmdDTO.Outputs))
		for _, outDTO := range cmdDTO.Outputs {
			out, err := dom.NewOutput(outDTO.Name, outDTO.Description, outDTO.Probe)
			if err != nil {
				return nil, fmt.Errorf("output inválido en comando '%s': %w", cmdDTO.Name, err)
			}
			outputs = append(outputs, out)
		}

		cmd, err := dom.NewCommand(
			cmdDTO.Name,
			cmdDTO.Cmd,
			dom.WithDescription(cmdDTO.Description),
			dom.WithWorkdir(cmdDTO.Workdir),
			dom.WithTemplateFiles(cmdDTO.TemplateFiles),
			dom.WithOutputs(outputs),
		)
		if err != nil {
			return nil, fmt.Errorf("comando inválido '%s': %w", cmdDTO.Name, err)
		}
		commands = append(commands, cmd)
	}
	return commands, nil
}

func (r *yamlFileReader) readVariablesFromFile(_ context.Context, variablesFilePath string) ([]dom.Variable, error) {
	data, err := os.ReadFile(variablesFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []dom.Variable{}, nil
		}
		return nil, err
	}

	var variableDTOs []dto.Variable
	if err := yaml.Unmarshal(data, &variableDTOs); err != nil {
		return nil, fmt.Errorf("parsear YAML de variables '%s': %w", variablesFilePath, err)
	}

	variables := make([]dom.Variable, 0, len(variableDTOs))
	for _, vDTO := range variableDTOs {
		variable, err := dom.NewVariable(vDTO.Name, vDTO.Value)
		if err != nil {
			return nil, fmt.Errorf("variable inválida '%s' en '%s': %w", vDTO.Name, variablesFilePath, err)
		}
		variables = append(variables, variable)
	}
	return variables, nil
}
