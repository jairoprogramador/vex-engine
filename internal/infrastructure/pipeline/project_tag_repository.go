package pipeline

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"sort"

	"github.com/Masterminds/semver/v3"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
)

var semverTagPattern = regexp.MustCompile(`^v?(\d+\.\d+\.\d+)$`)

type ProjectTagRepository struct {
}

var _ domPipeline.ProjectTagRepository = (*ProjectTagRepository)(nil)

func NewProjectTagRepository() domPipeline.ProjectTagRepository {
	return &ProjectTagRepository{}
}

func (r *ProjectTagRepository) LastTag(_ *context.Context, repositoryLocalPath string) (string, error) {
	repo, err := gogit.PlainOpen(repositoryLocalPath)
	if err != nil {
		return "", fmt.Errorf("abrir repositorio: %w", err)
	}
	tagIter, err := repo.Tags()
	if err != nil {
		return "", fmt.Errorf("obtener tags: %w", err)
	}
	var versions semver.Collection
	err = tagIter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		if semverTagPattern.MatchString(name) {
			if v, err := semver.NewVersion(name); err == nil {
				versions = append(versions, v)
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("iterar tags: %w", err)
	}
	if len(versions) == 0 {
		return "", nil
	}
	sort.Sort(versions)
	return versions[len(versions)-1].Original(), nil
}

func (r *ProjectTagRepository) RecentCommits(_ *context.Context, repositoryLocalPath string, sinceTag string, limit int) (headHash string, messages []string, err error) {
	repo, err := gogit.PlainOpen(repositoryLocalPath)
	if err != nil {
		return "", nil, fmt.Errorf("abrir repositorio: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return "", nil, fmt.Errorf("obtener HEAD: %w", err)
	}
	headHash = headRef.Hash().String()

	var stopHash plumbing.Hash
	if sinceTag != "" {
		stopHash, err = r.resolveTagCommitHash(repo, sinceTag)
		if err != nil {
			sinceTag = ""
		}
	}

	commitIter, err := repo.Log(&gogit.LogOptions{From: headRef.Hash()})
	if err != nil {
		return headHash, nil, fmt.Errorf("obtener log de commits: %w", err)
	}

	count := 0
	iterErr := commitIter.ForEach(func(c *object.Commit) error {
		if sinceTag != "" && c.Hash == stopHash {
			return io.EOF
		}

		messages = append(messages, c.Message)
		count++
		if count >= limit {
			return io.EOF
		}
		return nil
	})
	if iterErr != nil && iterErr != io.EOF {
		return headHash, nil, fmt.Errorf("iterar commits: %w", iterErr)
	}
	return headHash, messages, nil
}

func (r *ProjectTagRepository) resolveTagCommitHash(repo *gogit.Repository, tagName string) (plumbing.Hash, error) {
	ref, err := repo.Tag(tagName)
	if err != nil {
		ref, err = repo.Reference(plumbing.ReferenceName("refs/tags/"+tagName), true)
		if err != nil {
			return plumbing.ZeroHash, fmt.Errorf("tag '%s' no encontrado: %w", tagName, err)
		}
	}
	tagObj, err := repo.TagObject(ref.Hash())
	if err == nil {
		return tagObj.Target, nil
	}
	return ref.Hash(), nil
}
