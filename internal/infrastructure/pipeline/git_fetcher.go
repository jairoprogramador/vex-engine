package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/domain/pipeline/ports"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/git"
)

type GitFetcher struct {
	repositoryGit git.RepositoryGit
	baseDir       string
}

func NewGitFetcher(repositoryGit git.RepositoryGit, baseDir string) ports.RepositoryFetcher {
	return &GitFetcher{repositoryGit: repositoryGit, baseDir: baseDir}
}

var _ ports.RepositoryFetcher = (*GitFetcher)(nil)

func (f *GitFetcher) Fetch(ctx context.Context, url pipeline.RepositoryURL, ref pipeline.RepositoryRef) (string, error) {
	localPath := resolveLocalPath(f.baseDir, url)

	if err := os.MkdirAll(f.baseDir, 0o750); err != nil {
		return "", fmt.Errorf("git fetcher: crear directorio base: %w", err)
	}

	if err := f.repositoryGit.Clone(ctx, url.String(), ref.String(), localPath); err != nil {
		return "", fmt.Errorf("git fetcher: %w", err)
	}

	return localPath, nil
}
