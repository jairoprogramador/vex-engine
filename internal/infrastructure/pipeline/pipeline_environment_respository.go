package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline/dto"
	"gopkg.in/yaml.v3"
)

var _ domPipeline.PipelineEnvironmentRepository = (*PipelineEnvironmentRepository)(nil)

type PipelineEnvironmentRepository struct{}

func NewPipelineEnvironmentRepository() domPipeline.PipelineEnvironmentRepository {
	return &PipelineEnvironmentRepository{}
}

func (r *PipelineEnvironmentRepository) Get(_ *context.Context, pipelineLocalPath string) ([]string, error) {
	filePath := filepath.Join(pipelineLocalPath, "environments.yaml")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var dtos []dto.EnvironmentDTO
	if err := yaml.Unmarshal(data, &dtos); err != nil {
		return nil, fmt.Errorf("parsear YAML de entornos: %w", err)
	}

	values := make([]string, 0, len(dtos))
	for _, envDTO := range dtos {
		values = append(values, envDTO.Value)
	}
	return values, nil
}
