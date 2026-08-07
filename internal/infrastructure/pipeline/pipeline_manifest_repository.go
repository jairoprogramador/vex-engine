package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"gopkg.in/yaml.v3"
)

// PipelineManifestFileName es el manifiesto en la RAÍZ del pipelinecode. Se
// nombra aquí, y no en el dominio, porque es un detalle del formato en disco —el
// mismo reparto que `StepConfigFileName`.
const PipelineManifestFileName = "vexpipeline.yaml"

// PipelineManifestDTO es la forma en disco. Un solo campo hoy: la versión es de
// TODO el formato, así que lo que crece con las specs siguientes son los archivos
// que la versión cubre, no este DTO.
type PipelineManifestDTO struct {
	SchemaVersion int `yaml:"schema_version"`
}

type pipelineManifestRepository struct{}

var _ domPipeline.ManifestRepository = (*pipelineManifestRepository)(nil)

func NewPipelineManifestRepository() domPipeline.ManifestRepository {
	return &pipelineManifestRepository{}
}

// Get distingue tres situaciones, como el lector del `config.yaml` de un step:
//
//   - el archivo no está        ⇒ `NoManifest()`, sin error: versión 1.
//   - el archivo está y declara ⇒ el manifiesto con su versión.
//   - el archivo está y declara una versión que este motor no entiende ⇒ ERROR,
//     nombrando el archivo.
func (r *pipelineManifestRepository) Get(
	_ *context.Context, pipelineLocalPath string) (domPipeline.Manifest, error) {

	data, err := os.ReadFile(filepath.Join(pipelineLocalPath, PipelineManifestFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return domPipeline.NoManifest(), nil
		}
		return domPipeline.Manifest{}, fmt.Errorf("leer '%s': %w", PipelineManifestFileName, err)
	}

	var dto PipelineManifestDTO
	if err := yaml.Unmarshal(data, &dto); err != nil {
		return domPipeline.Manifest{}, fmt.Errorf(
			"parsear YAML de '%s': %w", PipelineManifestFileName, err)
	}

	manifest, err := domPipeline.NewManifest(dto.SchemaVersion)
	if err != nil {
		return domPipeline.Manifest{}, fmt.Errorf("'%s' %w", PipelineManifestFileName, err)
	}
	return manifest, nil
}
