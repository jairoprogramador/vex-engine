package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	execInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/execution"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo crea un repositorio git bare en un directorio temporal para simular un remoto.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	remoteDir := t.TempDir()
	tempCloneDir := t.TempDir()

	runCmd := func(dir string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		err := cmd.Run()
		require.NoError(t, err, "Command failed: %s %v", name, args)
	}

	// 1. Inicializar repositorio bare remoto
	runCmd(remoteDir, "git", "init", "--bare")
	runCmd(remoteDir, "git", "symbolic-ref", "HEAD", "refs/heads/develop")

	// 2. Clonar el repositorio bare a un directorio temporal de trabajo
	runCmd("", "git", "clone", remoteDir, tempCloneDir)

	// 3. Configurar el repo, crear y hacer commit de un archivo
	runCmd(tempCloneDir, "git", "config", "user.email", "test@example.com")
	runCmd(tempCloneDir, "git", "config", "user.name", "Test User")
	runCmd(tempCloneDir, "git", "checkout", "-b", "develop")
	err := os.WriteFile(filepath.Join(tempCloneDir, "README.md"), []byte("initial commit"), 0644)
	require.NoError(t, err)
	runCmd(tempCloneDir, "git", "add", "README.md")
	runCmd(tempCloneDir, "git", "commit", "-m", "chore: initial commit")

	// 4. Push de la rama al bare "remoto"
	runCmd(tempCloneDir, "git", "push", "-u", "origin", "develop")

	return remoteDir
}

func newTestCloner(t *testing.T) *git.GitCloner {
	t.Helper()
	runner := execInfra.NewShellCommandRunner()
	return git.NewGitCloner(runner)
}

func TestGitCloner_Clone(t *testing.T) {
	remoteRepoPath := setupTestRepo(t)
	cloner := newTestCloner(t)

	t.Run("should clone repository successfully when it does not exist", func(t *testing.T) {
		destDir := t.TempDir()
		clonePath := filepath.Join(destDir, "test-repo")

		err := cloner.Clone(context.Background(), remoteRepoPath, "develop", clonePath)

		require.NoError(t, err)
		_, err = os.Stat(filepath.Join(clonePath, ".git"))
		assert.NoError(t, err, ".git directory should exist after clone")
	})

	t.Run("should do nothing if repository already exists and is a git repo", func(t *testing.T) {
		destDir := t.TempDir()
		cmd := exec.Command("git", "clone", "--branch", "develop", remoteRepoPath, destDir)
		err := cmd.Run()
		require.NoError(t, err, "Pre-cloning for test failed")

		err = cloner.Clone(context.Background(), remoteRepoPath, "develop", destDir)

		require.NoError(t, err)
	})

	t.Run("should clone successfully into empty directory", func(t *testing.T) {
		destDir := t.TempDir()

		err := cloner.Clone(context.Background(), remoteRepoPath, "develop", destDir)

		require.NoError(t, err)
		_, statErr := os.Stat(filepath.Join(destDir, ".git"))
		assert.NoError(t, statErr, ".git should exist after cloning into an empty dir")
	})

	t.Run("should return an error for a non-existent branch", func(t *testing.T) {
		destDir := t.TempDir()

		err := cloner.Clone(context.Background(), remoteRepoPath, "non-existent-branch", destDir)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "git cloner: clonar")
	})
}
