package project

import (
	"context"
	"fmt"
	"os"

	"github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/project/dto"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/project/mapper"

	"gopkg.in/yaml.v3"
)

type YAMLProjectRepository struct{}

func NewYAMLProjectRepository() ports.ProjectRepository {
	return &YAMLProjectRepository{}
}

func (r *YAMLProjectRepository) Load(ctx context.Context, pathFile string) (*ports.ProjectConfigDTO, error) {
	data, err := os.ReadFile(pathFile)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el archivo de configuración '%s': %w", pathFile, err)
	}

	var dto dto.VexConfigDTO
	if err := yaml.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("error al parsear el archivo YAML de configuración: %w", err)
	}

	return mapper.ProjectToDomain(dto), nil
}

func (r *YAMLProjectRepository) Save(ctx context.Context, pathFile string, data *ports.ProjectConfigDTO) error {
	dto := mapper.ProjectToDto(data)

	yamlData, err := yaml.Marshal(&dto)
	if err != nil {
		return fmt.Errorf("error al serializar la configuración a YAML: %w", err)
	}

	if err := os.WriteFile(pathFile, yamlData, 0644); err != nil {
		return fmt.Errorf("no se pudo escribir el archivo de configuración actualizado '%s': %w", pathFile, err)
	}

	return nil
}
