package ports

import "context"

type PipelineCloner interface {
	Clone(ctx context.Context, repositoryUrl, ref string) (string, error)
}
