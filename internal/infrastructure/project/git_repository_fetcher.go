package project

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"

	"github.com/Masterminds/semver/v3"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	projDomain "github.com/jairoprogramador/vex-engine/internal/domain/project"
	"github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/git"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/repopath"
)

var semverTagPattern = regexp.MustCompile(`^v?(\d+\.\d+\.\d+)$`)

type GitRepositoryFetcher struct {
	cloner  git.RepositoryGit
	baseDir string
}

var _ ports.RepositoryFetcher = (*GitRepositoryFetcher)(nil)

func NewGitRepositoryFetcher(cloner git.RepositoryGit, baseDir string) ports.RepositoryFetcher {
	return &GitRepositoryFetcher{cloner: cloner, baseDir: baseDir}
}

func (r *GitRepositoryFetcher) Fetch(ctx context.Context, url projDomain.ProjectURL, ref projDomain.ProjectRef) (string, error) {
	localPath := repopath.ForClone(r.baseDir, url)
	if err := os.MkdirAll(r.baseDir, 0o750); err != nil {
		return "", fmt.Errorf("crear directorio base: %w", err)
	}
	if err := r.cloner.Clone(ctx, url.String(), ref.String(), localPath); err != nil {
		return "", fmt.Errorf("clonar repositorio del proyecto: %w", err)
	}
	return localPath, nil
}

func (r *GitRepositoryFetcher) LastTag(ctx context.Context, localPath string) (string, error) {
	repo, err := gogit.PlainOpen(localPath)
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

func (r *GitRepositoryFetcher) RecentCommits(ctx context.Context, localPath string, sinceTag string, limit int) (headHash string, messages []string, err error) {
	repo, err := gogit.PlainOpen(localPath)
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
		if count >= limit {
			return io.EOF
		}
		messages = append(messages, c.Message)
		count++
		return nil
	})
	if iterErr != nil && iterErr != io.EOF {
		return headHash, nil, fmt.Errorf("iterar commits: %w", iterErr)
	}
	return headHash, messages, nil
}

// proximo a eliminar
func (r *GitRepositoryFetcher) CreateTagForCommit(ctx context.Context, localPath, commitHash, tagName string) error {
	repo, err := gogit.PlainOpen(localPath)
	if err != nil {
		return fmt.Errorf("abrir repositorio: %w", err)
	}
	hash := plumbing.NewHash(commitHash)
	if _, err := repo.CommitObject(hash); err != nil {
		return fmt.Errorf("commit '%s' no encontrado: %w", commitHash, err)
	}
	tagRefName := plumbing.ReferenceName("refs/tags/" + tagName)
	if _, err := repo.Reference(tagRefName, true); err == nil {
		return fmt.Errorf("el tag '%s' ya existe", tagName)
	} else if err != plumbing.ErrReferenceNotFound {
		return fmt.Errorf("verificar tag '%s': %w", tagName, err)
	}
	ref := plumbing.NewHashReference(tagRefName, hash)
	if err := repo.Storer.SetReference(ref); err != nil {
		return fmt.Errorf("crear tag '%s': %w", tagName, err)
	}
	return nil
}

func (r *GitRepositoryFetcher) resolveTagCommitHash(repo *gogit.Repository, tagName string) (plumbing.Hash, error) {
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
