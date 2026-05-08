package step

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type VarsPipelineRepository interface {
	Get(ctx *context.Context, pipelineLocalPath, environment, step string) ([]command.Variable, error)
}
