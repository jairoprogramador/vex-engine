package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

var _ domPipeline.ProjectClonerRepository = (*LocalProjectClonerRepository)(nil)

// LocalProjectClonerRepository no clona: enlaza el directorio del proyecto ya
// presente en la máquina. `mountPoint` es ese directorio — en producción el
// volumen que monta el CLI (`/appProject`); el caller lo decide.
type LocalProjectClonerRepository struct {
	repositoryBasePath string
	mountPoint         string
}

func NewLocalProjectClonerRepository(repositoryBasePath, mountPoint string) domPipeline.ProjectClonerRepository {
	return &LocalProjectClonerRepository{
		repositoryBasePath: repositoryBasePath,
		mountPoint:         mountPoint,
	}
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

	if err := os.Symlink(r.mountPoint, localPath); err != nil {
		return "", fmt.Errorf("local project cloner: crear symlink '%s' → '%s': %w",
			localPath, r.mountPoint, err)
	}

	realPath, err := filepath.EvalSymlinks(localPath)
	if err != nil {
		return "", fmt.Errorf("local project cloner: resolver symlink '%s': %w", localPath, err)
	}

	return realPath, nil
}
