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
	proPorts "github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
)

// RepositoryGit is a consumer-side interface that abstracts the clone operation.
// Defined here (infrastructure side) so that application and pipeline packages
// can depend on this contract without importing a concrete type.
type RepositoryGit interface {
	Clone(ctx context.Context, repoURL, ref, localPath string) error
}

// pathLock serialises access to a single localPath. refs counts in-flight Clone
// calls that reserved this entry; when it reaches zero, the parent map drops the
// entry so a long-lived process does not accumulate mutexes for every path ever
// seen.
type pathLock struct {
	mu   sync.Mutex
	refs int
}

// RepositoryGitImpl ensures that a git repository is available at a local path.
// It uses go-git directly — no dependency on an external git binary.
//
// Concurrent calls for the same localPath are serialised via a per-path mutex so
// that two goroutines never race on the same directory.
type RepositoryGitImpl struct {
	mu    sync.Mutex
	locks map[string]*pathLock
}

// NewRepositoryGitImpl returns a RepositoryGitImpl ready for use.
func NewRepositoryGitImpl() *RepositoryGitImpl {
	return &RepositoryGitImpl{locks: make(map[string]*pathLock)}
}

// lockPath acquires the mutex for path and returns a release function.
// Callers must defer the returned function.
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

// cloneAction describes what Clone must do after inspecting the local path.
type cloneAction uint8

const (
	// cloneActionSkip — ref is an immutable full SHA; the working tree is already
	// at the correct commit. No network I/O needed.
	cloneActionSkip cloneAction = iota
	// cloneActionFresh — nothing exists at the path; clone directly.
	cloneActionFresh
	// cloneActionReplace — a stale/wrong directory exists; remove it first, then clone.
	cloneActionReplace
	// cloneActionUpdate — repo exists, origin URL and ref match, but the ref is a
	// mutable branch or tag. Fetch + hard-reset to pick up remote changes.
	cloneActionUpdate
)

// Clone guarantees that repoURL@ref is available at localPath and is up to date.
//
// Idempotency rules:
//   - Full 40-char SHA ref → skip entirely (immutable; already at the right commit).
//   - Mutable ref (branch, tag, refs/ path) → fetch + hard-reset so remote changes
//     are always reflected in the working tree.
//   - Missing or corrupt repo → clone from scratch.
//   - Origin URL mismatch or ref unknown locally → remove directory, then clone.
//
// When fetch fails (e.g. network error) the working copy is removed and a fresh
// clone is performed so callers always end up with a usable tree or an error.
//
// Concurrent calls for the same localPath are serialised internally; per-path
// lock entries are removed when no Clone is in flight for that path.
func (c *RepositoryGitImpl) Clone(ctx context.Context, repoURL, ref, localPath string) error {
	unlock := c.lockPath(localPath)
	defer unlock()

	action := resolveCloneAction(localPath, repoURL, ref)

	switch action {
	case cloneActionSkip:
		return nil

	case cloneActionUpdate:
		if err := fetchAndReset(ctx, localPath, ref); err != nil {
			// Network or other transient failure: fall back to a full reclone so
			// callers always get a usable working tree or an explicit error.
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

// fetchAndReset fetches the latest state of ref from origin and hard-resets the
// working tree to the fetched commit. It is only called for mutable refs (branches
// and tags), never for full SHAs.
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

	// Resolve the updated ref to its commit hash.
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		// Fallback for annotated tags in shallow clones: resolve via the tag ref
		// directly since the tag object itself may not be available.
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

// buildFetchRefSpec returns the appropriate RefSpec for fetching ref from origin.
// Tags use refs/tags/..., everything else (branches, full refs/ paths) uses
// refs/heads/... as a best-effort heuristic; the Force flag ensures the local
// ref is always overwritten with the remote state.
func buildFetchRefSpec(ref string) config.RefSpec {
	if strings.HasPrefix(ref, "refs/tags/") {
		return config.RefSpec(fmt.Sprintf("+%s:%s", ref, ref))
	}
	if strings.HasPrefix(ref, "refs/") {
		return config.RefSpec(fmt.Sprintf("+%s:%s", ref, ref))
	}
	// Heuristic: try as a tag ref first if it looks like a version string, but
	// go-git will simply fail the fetch if the remote does not have it. The caller
	// falls back to reclone on any fetch error, so this is safe.
	//
	// Default: treat as a branch.
	return config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/heads/%s", ref, ref))
}

// cloneWithRef tries to clone repoURL@ref at localPath.
//
// Resolution order:
//  1. If ref looks like a full refs/ path or a full 40-char hex SHA, use it verbatim.
//  2. Try as a branch.
//  3. On failure, retry as a tag.
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

	// If we already used an explicit ref path or SHA, do not retry.
	if isExplicitRef(ref) {
		return branchErr
	}

	// If the branch attempt failed and ref was treated as a branch, retry as a tag.
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
		// Return the original branch error — it is more informative.
		return branchErr
	}

	return branchErr
}

// hexSHARegexp matches only a full 40-character hex SHA. Short SHAs (7–39 chars)
// are intentionally excluded: go-git requires full SHAs when resolving references,
// and accepting short SHAs would produce silent failures or incorrect behaviour.
var hexSHARegexp = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// resolveReference maps a human-readable ref string to a go-git ReferenceName.
//
//   - Full refs/ path  → used verbatim
//   - Full 40-char hex SHA → used verbatim as an absolute reference
//   - Anything else    → treated as a branch name (caller may retry as tag)
func resolveReference(ref string) plumbing.ReferenceName {
	if strings.HasPrefix(ref, "refs/") {
		return plumbing.ReferenceName(ref)
	}
	if hexSHARegexp.MatchString(ref) {
		return plumbing.ReferenceName(ref)
	}
	return plumbing.NewBranchReferenceName(ref)
}

// isExplicitRef reports whether ref bypasses the branch/tag heuristic —
// i.e. it starts with "refs/" or looks like a full 40-char hex SHA.
func isExplicitRef(ref string) bool {
	return strings.HasPrefix(ref, "refs/") || hexSHARegexp.MatchString(ref)
}

// verifyWorkingTreeMatchesRef ensures HEAD points at the same commit ref resolves to.
// This rejects clone fallbacks that leave the default branch checked out when the
// requested ref does not exist.
func verifyWorkingTreeMatchesRef(localPath, ref string) error {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("verify clone: open: %w", err)
	}

	// For short refs, require a concrete local branch or tag so we do not accept
	// a successful PlainClone that fell back to the remote default branch.
	if !isExplicitRef(ref) {
		_, berr := repo.Reference(plumbing.NewBranchReferenceName(ref), false)
		_, terr := repo.Reference(plumbing.NewTagReferenceName(ref), false)
		if berr != nil && terr != nil {
			return fmt.Errorf("verify clone: ref %q does not exist as branch or tag", ref)
		}
	}

	expected, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		// Fallback for annotated tags in shallow clones: the tag object itself may
		// not be present, but the peeled tag reference is. Follow symrefs/peel to
		// reach the commit hash directly.
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

// resolveCloneAction inspects localPath and determines what Clone must do.
//
// Decision tree:
//
//	PlainOpen → ErrRepositoryNotExists       → cloneActionFresh   (no repo at path)
//	PlainOpen → any other error              → cloneActionReplace (invalid/corrupt repo)
//	origin URL mismatch (normalised)         → cloneActionReplace
//	ResolveRevision(ref) fails               → cloneActionReplace (ref unknown locally)
//	repo.Head() fails                        → cloneActionReplace
//	HEAD commit ≠ resolved ref commit        → cloneActionReplace
//	detached HEAD + ref names a branch only  → cloneActionReplace (re-checkout branch)
//	detached HEAD + explicit ref / tag ref   → cloneActionUpdate  (fetch + reset for tags/refs)
//	detached HEAD + full 40-char SHA         → cloneActionSkip    (immutable; already correct)
//	attached HEAD + mutable ref              → cloneActionUpdate  (fetch to pull remote changes)
//	attached HEAD + full 40-char SHA         → cloneActionSkip    (immutable; already correct)
func resolveCloneAction(localPath, repoURL, ref string) cloneAction {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		if errors.Is(err, gogit.ErrRepositoryNotExists) {
			return cloneActionFresh
		}
		// Directory exists but is not a valid git repository.
		return cloneActionReplace
	}

	// Verify the origin remote URL matches the requested one (normalised comparison).
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

	// HEAD is at the correct commit. Decide whether we can skip entirely or must
	// still fetch to pull potential remote changes.

	if !head.Name().IsBranch() {
		// Detached HEAD.
		if isExplicitRef(ref) {
			// Full 40-char SHA: immutable — no remote changes possible.
			if hexSHARegexp.MatchString(ref) {
				return cloneActionSkip
			}
			// refs/... path: mutable — fetch to stay current.
			return cloneActionUpdate
		}
		if _, err := repo.Reference(plumbing.NewTagReferenceName(ref), false); err == nil {
			// Tag ref: mutable — fetch to stay current.
			return cloneActionUpdate
		}
		// Detached on the right commit but ref names only a branch — reclone to
		// restore the branch checkout.
		return cloneActionReplace
	}

	// Attached HEAD on a branch. A full SHA is never a branch so this path always
	// implies a mutable ref. Fetch to pick up any new remote commits.
	if hexSHARegexp.MatchString(ref) {
		// Theoretically unreachable (a SHA cannot be a branch name), but guard
		// against it to be safe.
		return cloneActionSkip
	}
	return cloneActionUpdate
}

// originURL returns the fetch URL of the "origin" remote for repo.
// It returns an error if the remote is absent or has no configured URLs.
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

// scpStyleRegexp matches SCP-style SSH URLs: git@host:path/repo
// Capture groups: (1) host, (2) path
var scpStyleRegexp = regexp.MustCompile(`^[^@]+@([^:]+):(.+)$`)

// normalizeGitURL converts a git URL to a canonical lowercase form so that
// equivalent URLs that differ only in scheme, trailing ".git", trailing slash,
// or SCP-vs-HTTPS style compare as equal.
//
// Examples of URLs that normalise to the same value:
//
//	https://github.com/org/repo.git  →  github.com/org/repo
//	git@github.com:org/repo          →  github.com/org/repo
//	https://GitHub.com/Org/Repo/     →  github.com/org/repo
func normalizeGitURL(url string) string {
	url = strings.ToLower(strings.TrimSpace(url))

	// Convert SCP-style SSH (git@host:org/repo) to host/org/repo.
	if m := scpStyleRegexp.FindStringSubmatch(url); m != nil {
		url = m[1] + "/" + m[2]
	}

	// Strip common scheme prefixes.
	for _, prefix := range []string{"https://", "http://", "git://", "ssh://"} {
		if after, ok := strings.CutPrefix(url, prefix); ok {
			url = after
			break
		}
	}

	// Strip optional "user@" prefix that may remain after scheme removal.
	if idx := strings.Index(url, "@"); idx != -1 {
		url = url[idx+1:]
	}

	// Remove trailing slash and trailing ".git".
	url = strings.TrimRight(url, "/")
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimRight(url, "/") // trailing slash may re-appear after .git removal

	return url
}

// Compile-time contract verifications.
var _ RepositoryGit = (*RepositoryGitImpl)(nil)
var _ proPorts.ClonerTemplate = (*RepositoryGitImpl)(nil)
