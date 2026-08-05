package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
)

var _ domPipeline.PipelineStepRepository = (*PipelineStepRepository)(nil)

type PipelineStepRepository struct{}

func NewPipelineStepRepository() domPipeline.PipelineStepRepository {
	return &PipelineStepRepository{}
}

// Get devuelve el nombre de cada subdirectorio de `steps/` sin juzgarlo. Antes
// intentaba construir un StepName y descartaba en silencio el directorio cuyo
// nombre no casaba, así que un typo hacía desaparecer un step del pipeline sin
// un mensaje (spec 04 §1 c).
func (r *PipelineStepRepository) Get(_ *context.Context, pipelineLocalPath string) ([]domPipeline.StepEntry, error) {
	stepsPath := filepath.Join(pipelineLocalPath, "steps")

	files, err := os.ReadDir(stepsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []domPipeline.StepEntry{}, nil
		}
		return nil, fmt.Errorf("leer el directorio de steps '%s': %w", stepsPath, err)
	}

	entries := make([]domPipeline.StepEntry, 0, len(files))
	for _, file := range files {
		// Los archivos sueltos no son steps y nunca lo fueron: lo que un
		// pipelinecode declara es un directorio por step.
		if file.IsDir() {
			entries = append(entries, domPipeline.StepEntry(file.Name()))
		}
	}
	return entries, nil
}
