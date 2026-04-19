package ports

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/project"
)

type RepositoryFetcher interface {
	Fetch(ctx context.Context, url project.ProjectURL, ref project.ProjectRef) (localPath string, err error)
	LastTag(ctx context.Context, localPath string) (string, error)
	RecentCommits(ctx context.Context, localPath string, sinceTag string, limit int) (headHash string, messages []string, err error)
	//proximo a eliminar
	CreateTagForCommit(ctx context.Context, localPath, commitHash, tagName string) error
}
