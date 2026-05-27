package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

const localProjectMountPoint = "/appProject"

var _ domPipeline.ProjectClonerRepository = (*LocalProjectClonerRepository)(nil)

type LocalProjectClonerRepository struct {
	repositoryBasePath string
}

func NewLocalProjectClonerRepository(repositoryBasePath string) domPipeline.ProjectClonerRepository {
	return &LocalProjectClonerRepository{repositoryBasePath: repositoryBasePath}
}

func (r *LocalProjectClonerRepository) Clone(_ *context.Context, urlProject, _ string) (string, error) {
	projectName := utils.GetDirNameFromUrl(urlProject)
	localPath := filepath.Join(r.repositoryBasePath, projectName, "repository")

	if err := os.RemoveAll(localPath); err != nil {
		return "", fmt.Errorf("local project cloner: eliminar ruta previa '%s': %w", localPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o750); err != nil {
		return "", fmt.Errorf("local project cloner: crear directorio padre: %w", err)
	}

	if err := os.Symlink(localProjectMountPoint, localPath); err != nil {
		return "", fmt.Errorf("local project cloner: crear symlink '%s' → '%s': %w",
			localPath, localProjectMountPoint, err)
	}

	return localPath, nil
}
