package pipeline

import (
	"context"
)

type PipelineEnvironmentRepository interface {
	Get(ctx *context.Context, pipelineLocalPath string) ([]string, error)
}
