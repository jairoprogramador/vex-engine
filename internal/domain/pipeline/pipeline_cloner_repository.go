package pipeline

import (
	"context"
)

type PipelineClonerRepository interface {
	Clone(ctx *context.Context, urlPipeline, refPipeline string) (string, error)
}
