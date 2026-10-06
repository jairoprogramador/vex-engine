package infraestructura

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/jairoprogramador/vex-engine/internal/suministro/dominio"
)

// RepositoriosLocales es el acceso a los repositorios git de esta máquina, con go-git: una fuente es el
// directorio de su repositorio. El acceso a los remotos se compra en RD-11, detrás del mismo puerto.
//
// Cada material va a un directorio propio bajo base. Es una copia, y se puede borrar sin que cambie ninguna
// decisión (IT-05 DEC-05.10).
type RepositoriosLocales struct {
	base string
}

var _ dominio.Repositorios = (*RepositoriosLocales)(nil)

// prefijoMaterial marca los directorios que pone este acceso. Retirar no borra ningún otro.
const prefijoMaterial = "material-"

func NuevosRepositoriosLocales(base string) *RepositoriosLocales {
	return &RepositoriosLocales{base: base}
}

// PonerDeHoy pone delante el commit al que apunta la cabeza del repositorio. Lo que no tiene commit no está
// hoy: está en una copia de trabajo.
func (r *RepositoriosLocales) PonerDeHoy(ctx context.Context, fuente dominio.Fuente) (string, dominio.Commit, error) {
	repo, err := abrir(fuente)
	if err != nil {
		return "", dominio.Commit{}, err
	}
	cabeza, err := repo.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return "", dominio.Commit{}, fmt.Errorf("%w: la fuente %s no tiene ningún commit", dominio.ErrNoExiste, fuente)
	}
	if err != nil {
		return "", dominio.Commit{}, fmt.Errorf("repositorios locales: la cabeza de %s: %w", fuente, err)
	}
	commit, err := dominio.NuevoCommit(cabeza.Hash().String())
	if err != nil {
		return "", dominio.Commit{}, err
	}
	directorio, err := r.extraer(ctx, repo, fuente, cabeza.Hash())
	if err != nil {
		return "", dominio.Commit{}, err
	}
	return directorio, commit, nil
}

func (r *RepositoriosLocales) PonerDeUnCommit(
	ctx context.Context, fuente dominio.Fuente, commit dominio.Commit,
) (string, error) {
	if len(commit.String()) != 40 {
		return "", fmt.Errorf("repositorios locales: %s es un commit SHA-256, y go-git v5 solo lee SHA-1", commit)
	}
	repo, err := abrir(fuente)
	if err != nil {
		return "", err
	}
	return r.extraer(ctx, repo, fuente, plumbing.NewHash(commit.String()))
}

func (r *RepositoriosLocales) Retirar(_ context.Context, directorio string) error {
	base, err := filepath.Abs(r.base)
	if err != nil {
		return fmt.Errorf("repositorios locales: retirar %s: %w", directorio, err)
	}
	absoluto, err := filepath.Abs(directorio)
	if err != nil {
		return fmt.Errorf("repositorios locales: retirar %s: %w", directorio, err)
	}
	if filepath.Dir(absoluto) != base || !strings.HasPrefix(filepath.Base(absoluto), prefijoMaterial) {
		return fmt.Errorf("%w: %s no es un material que haya puesto este acceso", dominio.ErrInvalido, directorio)
	}
	if err := os.RemoveAll(absoluto); err != nil {
		return fmt.Errorf("repositorios locales: retirar %s: %w", directorio, err)
	}
	return nil
}

func abrir(fuente dominio.Fuente) (*git.Repository, error) {
	repo, err := git.PlainOpen(fuente.String())
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return nil, fmt.Errorf("%w: la fuente %s no es un repositorio", dominio.ErrNoExiste, fuente)
	}
	if err != nil {
		return nil, fmt.Errorf("repositorios locales: abrir %s: %w", fuente, err)
	}
	return repo, nil
}

// extraer escribe el árbol de un commit en un material nuevo: sus ficheros, con si son ejecutables, y sus
// enlaces. Un submódulo no entra: es un puntero a otro repositorio, no contenido.
func (r *RepositoriosLocales) extraer(
	ctx context.Context, repo *git.Repository, fuente dominio.Fuente, id plumbing.Hash,
) (string, error) {
	commit, err := repo.CommitObject(id)
	if errors.Is(err, plumbing.ErrObjectNotFound) {
		return "", fmt.Errorf("%w: la fuente %s no tiene el commit %s", dominio.ErrNoExiste, fuente, id)
	}
	if err != nil {
		return "", fmt.Errorf("repositorios locales: el commit %s de %s: %w", id, fuente, err)
	}
	arbol, err := commit.Tree()
	if err != nil {
		return "", fmt.Errorf("repositorios locales: el árbol del commit %s de %s: %w", id, fuente, err)
	}
	return r.poner(func(material *escritura) error {
		return arbol.Files().ForEach(func(f *object.File) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			switch f.Mode {
			case filemode.Symlink:
				destino, err := f.Contents()
				if err != nil {
					return err
				}
				return material.enlace(f.Name, destino)
			case filemode.Regular, filemode.Deprecated, filemode.Executable:
				contenido, err := f.Reader()
				if err != nil {
					return err
				}
				defer contenido.Close()
				return material.fichero(f.Name, f.Mode == filemode.Executable, contenido)
			}
			return nil
		})
	})
}

// poner crea un material nuevo y lo llena. Si llenarlo falla, lo borra: un material a medias no se entrega.
func (r *RepositoriosLocales) poner(llenar func(*escritura) error) (string, error) {
	if err := os.MkdirAll(r.base, 0o755); err != nil {
		return "", fmt.Errorf("repositorios locales: %w", err)
	}
	directorio, err := os.MkdirTemp(r.base, prefijoMaterial+"*")
	if err != nil {
		return "", fmt.Errorf("repositorios locales: %w", err)
	}
	if err := llenar(&escritura{raiz: directorio}); err != nil {
		return "", errors.Join(fmt.Errorf("repositorios locales: poner el material: %w", err), os.RemoveAll(directorio))
	}
	return directorio, nil
}
