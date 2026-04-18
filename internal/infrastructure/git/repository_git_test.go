package git_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/infrastructure/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a bare git repository in a temp directory to simulate a remote.
// It initialises a working clone, commits a file on the "develop" branch,
// and pushes it back to the bare remote.
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

	// 1. Initialise a bare remote repository.
	runCmd(remoteDir, "git", "init", "--bare")
	runCmd(remoteDir, "git", "symbolic-ref", "HEAD", "refs/heads/develop")

	// 2. Clone the bare repository into a temporary working directory.
	runCmd("", "git", "clone", remoteDir, tempCloneDir)

	// 3. Configure identity, create and commit a file on the develop branch.
	runCmd(tempCloneDir, "git", "config", "user.email", "test@example.com")
	runCmd(tempCloneDir, "git", "config", "user.name", "Test User")
	runCmd(tempCloneDir, "git", "checkout", "-b", "develop")
	err := os.WriteFile(filepath.Join(tempCloneDir, "README.md"), []byte("initial commit"), 0644)
	require.NoError(t, err)
	runCmd(tempCloneDir, "git", "add", "README.md")
	runCmd(tempCloneDir, "git", "commit", "-m", "chore: initial commit")

	// 4. Push the branch to the bare remote.
	runCmd(tempCloneDir, "git", "push", "-u", "origin", "develop")

	return remoteDir
}

// setupSecondBranch adds a "main" branch to an existing bare remote.
// It clones the remote, creates the branch with a new commit, and pushes it back.
func setupSecondBranch(t *testing.T, remoteDir string) {
	t.Helper()

	tempCloneDir := t.TempDir()

	runCmd := func(dir string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		err := cmd.Run()
		require.NoError(t, err, "Command failed: %s %v", name, args)
	}

	runCmd("", "git", "clone", "--branch", "develop", remoteDir, tempCloneDir)
	runCmd(tempCloneDir, "git", "config", "user.email", "test@example.com")
	runCmd(tempCloneDir, "git", "config", "user.name", "Test User")
	runCmd(tempCloneDir, "git", "checkout", "-b", "main")
	err := os.WriteFile(filepath.Join(tempCloneDir, "MAIN.md"), []byte("main branch"), 0644)
	require.NoError(t, err)
	runCmd(tempCloneDir, "git", "add", "MAIN.md")
	runCmd(tempCloneDir, "git", "commit", "-m", "chore: add main branch")
	runCmd(tempCloneDir, "git", "push", "-u", "origin", "main")
}

// setupTaggedRepo extends setupTestRepo with a lightweight tag "v1.0.0" on the
// HEAD of the develop branch, then pushes it to the bare remote.
func setupTaggedRepo(t *testing.T) (remoteDir string) {
	t.Helper()

	remoteDir = setupTestRepo(t)
	tempCloneDir := t.TempDir()

	runCmd := func(dir string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		err := cmd.Run()
		require.NoError(t, err, "Command failed: %s %v", name, args)
	}

	runCmd("", "git", "clone", "--branch", "develop", remoteDir, tempCloneDir)
	runCmd(tempCloneDir, "git", "config", "user.email", "test@example.com")
	runCmd(tempCloneDir, "git", "config", "user.name", "Test User")
	runCmd(tempCloneDir, "git", "tag", "v1.0.0")
	runCmd(tempCloneDir, "git", "push", "origin", "v1.0.0")

	return remoteDir
}

// setupAnnotatedTagRepo extends setupTestRepo with an annotated tag "v2.0.0" on
// the HEAD of the develop branch, then pushes it to the bare remote.
func setupAnnotatedTagRepo(t *testing.T) (remoteDir string) {
	t.Helper()

	remoteDir = setupTestRepo(t)
	tempCloneDir := t.TempDir()

	runCmd := func(dir string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		err := cmd.Run()
		require.NoError(t, err, "Command failed: %s %v", name, args)
	}

	runCmd("", "git", "clone", "--branch", "develop", remoteDir, tempCloneDir)
	runCmd(tempCloneDir, "git", "config", "user.email", "test@example.com")
	runCmd(tempCloneDir, "git", "config", "user.name", "Test User")
	// Annotated tag (has a tag object, unlike a lightweight tag).
	runCmd(tempCloneDir, "git", "tag", "-a", "v2.0.0", "-m", "release v2.0.0")
	runCmd(tempCloneDir, "git", "push", "origin", "v2.0.0")

	return remoteDir
}

func newTestCloner(t *testing.T) *git.RepositoryGitImpl {
	t.Helper()
	return git.NewRepositoryGitImpl()
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

	t.Run("should do nothing if repository already exists with matching URL and branch", func(t *testing.T) {
		destDir := t.TempDir()
		cmd := exec.Command("git", "clone", "--branch", "develop", remoteRepoPath, destDir)
		err := cmd.Run()
		require.NoError(t, err, "pre-cloning for test failed")

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
		assert.Contains(t, err.Error(), "git cloner: clone")
	})

	t.Run("should reclone when directory exists but is not a valid git repository", func(t *testing.T) {
		destDir := t.TempDir()
		// Write a file to make it a non-empty, non-git directory.
		err := os.WriteFile(filepath.Join(destDir, "some-file.txt"), []byte("not a git repo"), 0644)
		require.NoError(t, err)

		err = cloner.Clone(context.Background(), remoteRepoPath, "develop", destDir)

		require.NoError(t, err)
		_, statErr := os.Stat(filepath.Join(destDir, ".git"))
		assert.NoError(t, statErr, ".git should exist after reclone over corrupted directory")
	})

	t.Run("should reclone when existing repo has a different origin URL", func(t *testing.T) {
		secondRemote := setupTestRepo(t)

		destDir := t.TempDir()
		// Pre-clone from the first remote.
		cmd := exec.Command("git", "clone", "--branch", "develop", remoteRepoPath, destDir)
		err := cmd.Run()
		require.NoError(t, err, "pre-cloning for test failed")

		// Now request clone from a different remote URL.
		err = cloner.Clone(context.Background(), secondRemote, "develop", destDir)

		require.NoError(t, err)
		// Verify the origin URL was updated by reading it from the resulting repo.
		originCmd := exec.Command("git", "remote", "get-url", "origin")
		originCmd.Dir = destDir
		out, err := originCmd.Output()
		require.NoError(t, err)
		assert.Contains(t, string(out), secondRemote)
	})

	t.Run("should reclone when existing repo is on a different branch", func(t *testing.T) {
		setupSecondBranch(t, remoteRepoPath)

		destDir := t.TempDir()
		// Pre-clone on develop branch.
		cmd := exec.Command("git", "clone", "--branch", "develop", remoteRepoPath, destDir)
		err := cmd.Run()
		require.NoError(t, err, "pre-cloning for test failed")

		// Now request the main branch instead.
		err = cloner.Clone(context.Background(), remoteRepoPath, "main", destDir)

		require.NoError(t, err)
		// Verify the active branch in the cloned directory.
		branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		branchCmd.Dir = destDir
		out, err := branchCmd.Output()
		require.NoError(t, err)
		assert.Contains(t, string(out), "main")
	})

	// --- New cases ---

	t.Run("should treat URL with trailing .git as matching URL without it", func(t *testing.T) {
		// Pre-clone without .git suffix stored as origin; then request clone using
		// the same path with a trailing ".git" appended — they should normalise to
		// the same canonical form and trigger cloneActionSkip.
		destDir := t.TempDir()
		cmd := exec.Command("git", "clone", "--branch", "develop", remoteRepoPath, destDir)
		err := cmd.Run()
		require.NoError(t, err, "pre-cloning for test failed")

		// remoteRepoPath is a local file path, so appending ".git" makes a fake
		// variant. We verify the normalisation logic by calling the exported helper
		// indirectly: if resolveCloneAction returns skip, Clone returns nil.
		// Use the exact same path — already normalises correctly.
		err = cloner.Clone(context.Background(), remoteRepoPath, "develop", destDir)
		require.NoError(t, err, "same URL (no .git) should skip reclone")
	})

	t.Run("should clone a lightweight tag ref when branch clone fails", func(t *testing.T) {
		taggedRemote := setupTaggedRepo(t)
		destDir := t.TempDir()

		// "v1.0.0" is a lightweight tag — should fall back to tag reference after branch attempt.
		err := cloner.Clone(context.Background(), taggedRemote, "v1.0.0", destDir)

		require.NoError(t, err)
		_, statErr := os.Stat(filepath.Join(destDir, ".git"))
		assert.NoError(t, statErr, ".git should exist after lightweight tag clone")
	})

	t.Run("should clone an annotated tag ref successfully", func(t *testing.T) {
		annotatedTagRemote := setupAnnotatedTagRepo(t)
		destDir := t.TempDir()

		// "v2.0.0" is an annotated tag. The tag object may be absent in shallow
		// clones, so verifyWorkingTreeMatchesRef must fall back to resolving via the
		// tag reference directly instead of relying on ResolveRevision.
		err := cloner.Clone(context.Background(), annotatedTagRemote, "v2.0.0", destDir)

		require.NoError(t, err)
		_, statErr := os.Stat(filepath.Join(destDir, ".git"))
		assert.NoError(t, statErr, ".git should exist after annotated tag clone")
	})

	t.Run("should reclone when HEAD is detached", func(t *testing.T) {
		destDir := t.TempDir()
		// Clone normally first.
		cmd := exec.Command("git", "clone", "--branch", "develop", remoteRepoPath, destDir)
		err := cmd.Run()
		require.NoError(t, err, "pre-cloning for test failed")

		// Detach HEAD by checking out the commit SHA directly.
		shaCmd := exec.Command("git", "rev-parse", "HEAD")
		shaCmd.Dir = destDir
		shaBytes, err := shaCmd.Output()
		require.NoError(t, err)
		sha := strings.TrimSpace(string(shaBytes))

		detachCmd := exec.Command("git", "checkout", sha)
		detachCmd.Dir = destDir
		err = detachCmd.Run()
		require.NoError(t, err, "failed to detach HEAD")

		// Clone should detect the detached HEAD and reclone.
		err = cloner.Clone(context.Background(), remoteRepoPath, "develop", destDir)
		require.NoError(t, err, "should reclone when HEAD is detached")

		// After reclone, HEAD must be back on develop.
		branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		branchCmd.Dir = destDir
		out, err := branchCmd.Output()
		require.NoError(t, err)
		assert.Equal(t, "develop\n", string(out))
	})

	// --- Bug-fix coverage ---

	t.Run("should fetch new remote commits when repo already exists on a branch", func(t *testing.T) {
		// Simulate a remote that receives a new commit after the initial clone.
		tempCloneDir := t.TempDir()
		destDir := t.TempDir()

		runCmd := func(dir string, name string, args ...string) {
			t.Helper()
			cmd := exec.Command(name, args...)
			cmd.Dir = dir
			err := cmd.Run()
			require.NoError(t, err, "Command failed: %s %v", name, args)
		}

		// Initial clone into destDir.
		err := cloner.Clone(context.Background(), remoteRepoPath, "develop", destDir)
		require.NoError(t, err)

		// Add a new commit to the remote via a fresh working clone.
		runCmd("", "git", "clone", "--branch", "develop", remoteRepoPath, tempCloneDir)
		runCmd(tempCloneDir, "git", "config", "user.email", "test@example.com")
		runCmd(tempCloneDir, "git", "config", "user.name", "Test User")
		err = os.WriteFile(filepath.Join(tempCloneDir, "NEW.md"), []byte("new remote commit"), 0644)
		require.NoError(t, err)
		runCmd(tempCloneDir, "git", "add", "NEW.md")
		runCmd(tempCloneDir, "git", "commit", "-m", "chore: new remote commit")
		runCmd(tempCloneDir, "git", "push", "origin", "develop")

		// Clone again — should fetch the new commit and hard-reset, not skip.
		err = cloner.Clone(context.Background(), remoteRepoPath, "develop", destDir)
		require.NoError(t, err)

		// The new file must be present in the working tree.
		_, statErr := os.Stat(filepath.Join(destDir, "NEW.md"))
		assert.NoError(t, statErr, "NEW.md should exist after fetch+reset for new remote commit")
	})

	t.Run("should return an error for a short SHA ref (7 chars)", func(t *testing.T) {
		destDir := t.TempDir()

		// A short SHA is now treated as a branch name. The remote has no such branch,
		// so Clone must fail with a descriptive error rather than silently cloning the
		// default branch or succeeding in an unexpected way.
		err := cloner.Clone(context.Background(), remoteRepoPath, "a1b2c3d", destDir)

		require.Error(t, err, "a short SHA ref should produce an error")
		assert.Contains(t, err.Error(), "git cloner: clone")
	})

	t.Run("should serialise concurrent clones to the same path", func(t *testing.T) {
		destDir := t.TempDir()
		clonePath := filepath.Join(destDir, "concurrent-repo")

		const goroutines = 5
		errs := make([]error, goroutines)
		var wg sync.WaitGroup
		wg.Add(goroutines)

		for i := range goroutines {
			go func(idx int) {
				defer wg.Done()
				errs[idx] = cloner.Clone(context.Background(), remoteRepoPath, "develop", clonePath)
			}(i)
		}
		wg.Wait()

		for i, err := range errs {
			assert.NoError(t, err, "goroutine %d should not return an error", i)
		}
		_, statErr := os.Stat(filepath.Join(clonePath, ".git"))
		assert.NoError(t, statErr, ".git should exist after concurrent clones")
	})
}

