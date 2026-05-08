package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

type repositoryURL string

func (u repositoryURL) Name() string   { return string(u) }
func (u repositoryURL) String() string { return string(u) }

var _ domPipeline.PipelineClonerRepository = (*PipelineClonerRepository)(nil)

type PipelineClonerRepository struct {
	repositoryBasePath string
}

func NewPipelineClonerRepository(repositoryBasePath string) domPipeline.PipelineClonerRepository {
	return &PipelineClonerRepository{repositoryBasePath: repositoryBasePath}
}

func (r *PipelineClonerRepository) Clone(ctx *context.Context, urlProject, urlPipeline, refPipeline string) (string, error) {

	projectName := utils.GetDirNameFromUrl(urlProject)
	pipelineName := utils.GetDirNameFromUrl(urlPipeline)

	localPath := filepath.Join(r.repositoryBasePath, projectName, "pipelines", pipelineName)

	if err := os.RemoveAll(localPath); err != nil {
		return "", fmt.Errorf("pipeline cloner repository: eliminar ruta previa '%s': %w", localPath, err)
	}

	if err := os.MkdirAll(localPath, 0o750); err != nil {
		return "", fmt.Errorf("pipeline cloner repository: crear base '%s': %w", localPath, err)
	}

	if err := cloneWithRef(*ctx, urlPipeline, refPipeline, localPath, 1); err != nil {
		removeDirIfEmpty(localPath)
		return "", fmt.Errorf("pipeline cloner repository: clonar ref %q: %w", refPipeline, err)
	}

	return localPath, nil
}
