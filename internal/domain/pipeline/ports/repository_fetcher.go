package ports

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
)

type RepositoryFetcher interface {
	Fetch(ctx context.Context, url pipeline.PipelineURL, ref pipeline.PipelineRef) (string, error)
}
