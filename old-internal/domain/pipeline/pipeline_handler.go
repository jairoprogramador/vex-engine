package pipeline

import (
	"context"
)

type PipelineHandler interface {
	SetNext(next PipelineHandler)
	Handle(ctx *context.Context, request *PipelineRequestHandler) error
}
