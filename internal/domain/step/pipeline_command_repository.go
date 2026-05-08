package step

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type PipelineCommandRepository interface {
	Get(ctx *context.Context, pipelineLocalPath, step string) ([]command.Command, error)
}
