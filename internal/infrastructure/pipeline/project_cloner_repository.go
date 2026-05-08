package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

var _ domPipeline.ProjectClonerRepository = (*ProjectClonerRepository)(nil)

type ProjectClonerRepository struct {
	repositoryBasePath string
}

func NewProjectClonerRepository(repositoryBasePath string) domPipeline.ProjectClonerRepository {
	return &ProjectClonerRepository{repositoryBasePath: repositoryBasePath}
}

func (r *ProjectClonerRepository) Clone(ctx *context.Context, urlProject, refProject string) (string, error) {
	projectName := utils.GetDirNameFromUrl(urlProject)

	localPath := filepath.Join(r.repositoryBasePath, projectName, "repository")

	if err := os.RemoveAll(localPath); err != nil {
		return "", fmt.Errorf("project cloner repository: eliminar ruta previa '%s': %w", localPath, err)
	}

	if err := os.MkdirAll(localPath, 0o750); err != nil {
		return "", fmt.Errorf("project cloner repository: crear base '%s': %w", localPath, err)
	}

	if err := cloneWithRef(*ctx, urlProject, refProject, localPath, domPipeline.MaxCommitsForVersioning+1); err != nil {
		removeDirIfEmpty(localPath)
		return "", fmt.Errorf("project cloner repository: clonar ref %q: %w", refProject, err)
	}

	return localPath, nil
}
