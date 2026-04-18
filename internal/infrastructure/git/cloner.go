package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	execPorts "github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
)

// gitCloner es una interfaz local que abstrae la operación de clonación.
// Se define aquí (lado del consumidor) para romper el acoplamiento con domain/project.
// infrastructure/git.GitCloner la satisface implícitamente.
type RepositoryGit interface {
	Clone(ctx context.Context, repoURL, ref, localPath string) error
}

// GitCloner garantiza que un repositorio git esté disponible en una ruta local.
// Implementa una única responsabilidad: clonar si no existe, no-op si ya existe.
type GitCloner struct {
	runner execPorts.CommandRunner
}

func NewGitCloner(runner execPorts.CommandRunner) *GitCloner {
	return &GitCloner{runner: runner}
}

// EnsureCloned garantiza que repoURL@ref esté disponible en localPath.
// Si el directorio ya existe y contiene un repositorio git válido, no hace nada.
func (c *GitCloner) Clone(ctx context.Context, repoURL, ref, localPath string) error {
	if _, err := os.Stat(localPath); err == nil {
		isGit, err := isGitRepository(localPath)
		if err != nil {
			return fmt.Errorf("git cloner: verificar repositorio: %w", err)
		}
		if isGit {
			return nil
		}
	}

	command := fmt.Sprintf("git clone --branch %s %s %s", ref, repoURL, localPath)
	res, err := c.runner.Run(ctx, command, "")
	if err != nil {
		return fmt.Errorf("git cloner: iniciar clonación: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("git cloner: clonar '%s' (código %d): %s", repoURL, res.ExitCode, res.CombinedOutput())
	}
	return nil
}

// Verificación de contrato en compile-time.
var _ RepositoryGit = (*GitCloner)(nil)

func isGitRepository(path string) (bool, error) {
	_, err := os.Stat(filepath.Join(path, ".git"))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
