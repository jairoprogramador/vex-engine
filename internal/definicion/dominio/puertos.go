package dominio

import "context"

type Pipelines interface {
	DeHoy(ctx context.Context, fuente string) (*PipelineComprobado, error)
	DeUnCommit(ctx context.Context, fuente, commit string) (*PipelineComprobado, error)
}
