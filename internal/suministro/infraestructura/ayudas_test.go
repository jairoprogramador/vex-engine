package infraestructura_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"
)

var cuando = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

func escribir(t *testing.T, raiz, ruta, contenido string) {
	t.Helper()
	camino := filepath.Join(raiz, filepath.FromSlash(ruta))
	require.NoError(t, os.MkdirAll(filepath.Dir(camino), 0o755))
	require.NoError(t, os.WriteFile(camino, []byte(contenido), 0o644))
}

func leer(t *testing.T, raiz, ruta string) string {
	t.Helper()
	contenido, err := os.ReadFile(filepath.Join(raiz, filepath.FromSlash(ruta)))
	require.NoError(t, err)
	return string(contenido)
}

func repositorio(t *testing.T) (string, *git.Repository) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	return dir, repo
}

// commitear añade todo lo que no está ignorado, como git add -A, y hace un commit.
func commitear(t *testing.T, repo *git.Repository, mensaje string) string {
	t.Helper()
	w, err := repo.Worktree()
	require.NoError(t, err)
	require.NoError(t, w.AddWithOptions(&git.AddOptions{All: true}))
	id, err := w.Commit(mensaje, &git.CommitOptions{
		Author: &object.Signature{Name: "ana", Email: "ana@vex.test", When: cuando},
	})
	require.NoError(t, err)
	return id.String()
}

// rutas son los ficheros y enlaces de un directorio, con '/', en orden.
func rutas(t *testing.T, raiz string) []string {
	t.Helper()
	var todas []string
	require.NoError(t, filepath.WalkDir(raiz, func(camino string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relativa, err := filepath.Rel(raiz, camino)
		todas = append(todas, filepath.ToSlash(relativa))
		return err
	}))
	sort.Strings(todas)
	return todas
}
