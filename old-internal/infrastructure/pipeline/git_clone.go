package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

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

func cloneWithRef(ctx context.Context, repoURL, ref, localPath string, depth int) error {
	refName := resolveReference(ref)

	_, err := gogit.PlainCloneContext(ctx, localPath, false, &gogit.CloneOptions{
		URL:           repoURL,
		ReferenceName: refName,
		SingleBranch:  true,
		Depth:         depth,
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
			return fmt.Errorf("remove partial clone attempt: %w", rmErr)
		}

		_, retryErr := gogit.PlainCloneContext(ctx, localPath, false, &gogit.CloneOptions{
			URL:           repoURL,
			ReferenceName: plumbing.NewTagReferenceName(ref),
			SingleBranch:  true,
			Depth:         depth,
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

func removeDirIfEmpty(path string) {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(path)
}
