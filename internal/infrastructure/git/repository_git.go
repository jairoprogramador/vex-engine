package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

type RepositoryGit interface {
	Clone(ctx context.Context, repoURL, ref, localPath string) error
}

type pathLock struct {
	mu   sync.Mutex
	refs int
}

type RepositoryGitImpl struct {
	mu    sync.Mutex
	locks map[string]*pathLock
}

func NewRepositoryGitImpl() *RepositoryGitImpl {
	return &RepositoryGitImpl{locks: make(map[string]*pathLock)}
}

func (c *RepositoryGitImpl) lockPath(path string) func() {
	c.mu.Lock()
	entry, ok := c.locks[path]
	if !ok {
		entry = &pathLock{}
		c.locks[path] = entry
	}
	entry.refs++
	c.mu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		c.mu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(c.locks, path)
		}
		c.mu.Unlock()
	}
}

type cloneAction uint8

const (
	cloneActionSkip cloneAction = iota
	cloneActionFresh
	cloneActionReplace
	cloneActionUpdate
)

func (c *RepositoryGitImpl) Clone(ctx context.Context, repoURL, ref, localPath string) error {
	unlock := c.lockPath(localPath)
	defer unlock()

	action := resolveCloneAction(localPath, repoURL, ref)

	switch action {
	case cloneActionSkip:
		return nil

	case cloneActionUpdate:
		if err := fetchAndReset(ctx, localPath, ref); err != nil {
			if rmErr := os.RemoveAll(localPath); rmErr != nil {
				return fmt.Errorf("git cloner: remove for reclone after fetch failure: %w", rmErr)
			}
			if cloneErr := cloneWithRef(ctx, repoURL, ref, localPath); cloneErr != nil {
				return fmt.Errorf("git cloner: clone '%s' ref '%s': %w", repoURL, ref, cloneErr)
			}
		}
		return nil

	case cloneActionReplace:
		if err := os.RemoveAll(localPath); err != nil {
			return fmt.Errorf("git cloner: remove stale directory '%s': %w", localPath, err)
		}
	}

	if err := cloneWithRef(ctx, repoURL, ref, localPath); err != nil {
		return fmt.Errorf("git cloner: clone '%s' ref '%s': %w", repoURL, ref, err)
	}
	return nil
}

func fetchAndReset(ctx context.Context, localPath, ref string) error {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("git cloner: fetch and reset: open repo: %w", err)
	}

	refSpec := buildFetchRefSpec(ref)
	fetchErr := repo.FetchContext(ctx, &gogit.FetchOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{refSpec},
		Depth:      1,
		Force:      true,
	})
	if fetchErr != nil && !errors.Is(fetchErr, gogit.NoErrAlreadyUpToDate) {
		return fmt.Errorf("git cloner: fetch and reset: fetch: %w", fetchErr)
	}

	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		tagRef, tagErr := repo.Reference(plumbing.NewTagReferenceName(ref), true)
		if tagErr != nil {
			return fmt.Errorf("git cloner: fetch and reset: resolve ref %q: %w", ref, err)
		}
		h := tagRef.Hash()
		hash = &h
	}

	w, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("git cloner: fetch and reset: worktree: %w", err)
	}
	if err := w.Reset(&gogit.ResetOptions{
		Commit: *hash,
		Mode:   gogit.HardReset,
	}); err != nil {
		return fmt.Errorf("git cloner: fetch and reset: hard reset: %w", err)
	}
	return nil
}

func buildFetchRefSpec(ref string) config.RefSpec {
	if strings.HasPrefix(ref, "refs/tags/") {
		return config.RefSpec(fmt.Sprintf("+%s:%s", ref, ref))
	}
	if strings.HasPrefix(ref, "refs/") {
		return config.RefSpec(fmt.Sprintf("+%s:%s", ref, ref))
	}
	return config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/heads/%s", ref, ref))
}

func cloneWithRef(ctx context.Context, repoURL, ref, localPath string) error {
	refName := resolveReference(ref)

	_, err := gogit.PlainCloneContext(ctx, localPath, false, &gogit.CloneOptions{
		URL:           repoURL,
		ReferenceName: refName,
		SingleBranch:  true,
		Depth:         1,
	})
	if err == nil {
		if verr := verifyWorkingTreeMatchesRef(localPath, ref); verr != nil {
			if rmErr := os.RemoveAll(localPath); rmErr != nil {
				return fmt.Errorf("%w (cleanup: %v)", verr, rmErr)
			}
			return verr
		}
		return nil
	}

	branchErr := err

	if isExplicitRef(ref) {
		return branchErr
	}

	if refName == plumbing.NewBranchReferenceName(ref) {
		if rmErr := os.RemoveAll(localPath); rmErr != nil {
			return fmt.Errorf("git cloner: remove partial clone attempt: %w", rmErr)
		}

		_, retryErr := gogit.PlainCloneContext(ctx, localPath, false, &gogit.CloneOptions{
			URL:           repoURL,
			ReferenceName: plumbing.NewTagReferenceName(ref),
			SingleBranch:  true,
			Depth:         1,
		})
		if retryErr == nil {
			if verr := verifyWorkingTreeMatchesRef(localPath, ref); verr != nil {
				if rmErr := os.RemoveAll(localPath); rmErr != nil {
					return fmt.Errorf("%w (cleanup: %v)", verr, rmErr)
				}
				return verr
			}
			return nil
		}
		return branchErr
	}

	return branchErr
}

var hexSHARegexp = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

func resolveReference(ref string) plumbing.ReferenceName {
	if strings.HasPrefix(ref, "refs/") {
		return plumbing.ReferenceName(ref)
	}
	if hexSHARegexp.MatchString(ref) {
		return plumbing.ReferenceName(ref)
	}
	return plumbing.NewBranchReferenceName(ref)
}

func isExplicitRef(ref string) bool {
	return strings.HasPrefix(ref, "refs/") || hexSHARegexp.MatchString(ref)
}

func verifyWorkingTreeMatchesRef(localPath, ref string) error {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("verify clone: open: %w", err)
	}

	if !isExplicitRef(ref) {
		_, berr := repo.Reference(plumbing.NewBranchReferenceName(ref), false)
		_, terr := repo.Reference(plumbing.NewTagReferenceName(ref), false)
		if berr != nil && terr != nil {
			return fmt.Errorf("verify clone: ref %q does not exist as branch or tag", ref)
		}
	}

	expected, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			tagRef, tagErr := repo.Reference(plumbing.NewTagReferenceName(ref), true)
			if tagErr == nil {
				h := tagRef.Hash()
				expected = &h
				err = nil
			}
		}
		if err != nil {
			return fmt.Errorf("verify clone: resolve ref %q: %w", ref, err)
		}
	}

	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("verify clone: head: %w", err)
	}
	if head.Hash() != *expected {
		return fmt.Errorf("verify clone: HEAD %s does not match ref %q (%s)", head.Hash(), ref, expected)
	}
	return nil
}

func resolveCloneAction(localPath, repoURL, ref string) cloneAction {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return cloneActionFresh
		}
		return cloneActionReplace
	}

	remoteURL, err := originURL(repo)
	if err != nil || normalizeGitURL(remoteURL) != normalizeGitURL(repoURL) {
		return cloneActionReplace
	}

	expected, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return cloneActionReplace
	}

	head, err := repo.Head()
	if err != nil {
		return cloneActionReplace
	}

	if head.Hash() != *expected {
		return cloneActionReplace
	}

	if !head.Name().IsBranch() {
		if isExplicitRef(ref) {
			if hexSHARegexp.MatchString(ref) {
				return cloneActionSkip
			}
			return cloneActionUpdate
		}
		if _, err := repo.Reference(plumbing.NewTagReferenceName(ref), false); err == nil {
			return cloneActionUpdate
		}
		return cloneActionReplace
	}

	if hexSHARegexp.MatchString(ref) {
		return cloneActionSkip
	}
	return cloneActionUpdate
}

func originURL(repo *gogit.Repository) (string, error) {
	remotes, err := repo.Remotes()
	if err != nil {
		return "", fmt.Errorf("list remotes: %w", err)
	}

	for _, r := range remotes {
		if r.Config().Name == "origin" {
			urls := r.Config().URLs
			if len(urls) == 0 {
				return "", fmt.Errorf("remote 'origin' has no configured URLs")
			}
			return urls[0], nil
		}
	}
	return "", fmt.Errorf("remote 'origin' not found")
}

var scpStyleRegexp = regexp.MustCompile(`^[^@]+@([^:]+):(.+)$`)

func normalizeGitURL(url string) string {
	url = strings.ToLower(strings.TrimSpace(url))

	if m := scpStyleRegexp.FindStringSubmatch(url); m != nil {
		url = m[1] + "/" + m[2]
	}

	for _, prefix := range []string{"https://", "http://", "git://", "ssh://"} {
		if after, ok := strings.CutPrefix(url, prefix); ok {
			url = after
			break
		}
	}

	if idx := strings.Index(url, "@"); idx != -1 {
		url = url[idx+1:]
	}

	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimRight(url, "/")

	return url
}

var _ RepositoryGit = (*RepositoryGitImpl)(nil)
