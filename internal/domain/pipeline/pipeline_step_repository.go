package pipeline

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type PipelineStepRepository interface {
	Get(ctx *context.Context, pipelineLocalPath string) ([]command.StepName, error)
}
