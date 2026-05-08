package pipeline

import (
	"context"
)

type PipelineClonerRepository interface {
	Clone(ctx *context.Context, urlProject, urlPipeline, refPipeline string) (string, error)
}
