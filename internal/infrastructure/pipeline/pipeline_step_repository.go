package pipeline

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
)

var _ domPipeline.PipelineStepRepository = (*PipelineStepRepository)(nil)

type PipelineStepRepository struct{}

func NewPipelineStepRepository() domPipeline.PipelineStepRepository {
	return &PipelineStepRepository{}
}

func (r *PipelineStepRepository) Get(_ *context.Context, pipelineLocalPath string) ([]command.StepName, error) {
	stepsPath := filepath.Join(pipelineLocalPath, "steps")

	files, err := os.ReadDir(stepsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []command.StepName{}, nil
		}
		return nil, err
	}

	stepNames := make([]command.StepName, 0)
	for _, file := range files {
		if file.IsDir() {
			stepName, err := command.NewStepName(file.Name())
			if err == nil {
				stepNames = append(stepNames, stepName)
			}
		}
	}
	return stepNames, nil
}
