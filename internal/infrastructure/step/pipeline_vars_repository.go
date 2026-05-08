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

type pipelineVarsRepository struct{}

func NewPipelineVarsRepository() domStep.VarsPipelineRepository {
	return &pipelineVarsRepository{}
}

func (r *pipelineVarsRepository) Get(ctx *context.Context, pipelineLocalPath, environment, step string) ([]command.Variable, error) {
	return r.readVariablesFromFile(*ctx, filepath.Join(pipelineLocalPath, "variables", environment, step+".yaml"))
}

func (r *pipelineVarsRepository) readVariablesFromFile(_ context.Context, filePath string) ([]command.Variable, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []command.Variable{}, nil
		}
		return nil, err
	}

	var variablesDTO []PipelineVariableDTO
	if err := yaml.Unmarshal(data, &variablesDTO); err != nil {
		return nil, fmt.Errorf("parsear YAML de variables '%s': %w", filePath, err)
	}

	variables := make([]command.Variable, 0, len(variablesDTO))
	for _, vDTO := range variablesDTO {
		variable, err := command.NewVariable(vDTO.Name, vDTO.Value, false)
		if err != nil {
			return nil, fmt.Errorf("variable inválida '%s' en '%s': %w", vDTO.Name, filePath, err)
		}
		variables = append(variables, variable)
	}
	return variables, nil
}
