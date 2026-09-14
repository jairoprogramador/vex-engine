package pipeline

import (
	"context"
)

type ProjectTagRepository interface {
	LastTag(ctx *context.Context, repositoryLocalPath string) (string, error)
	RecentCommits(ctx *context.Context, repositoryLocalPath string, sinceTag string, limit int) (headHash string, messages []string, err error)
}
