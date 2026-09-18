package infraestructura_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/definicion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/definicion/infraestructura"
	suministro "github.com/jairoprogramador/vex-engine/internal/suministro/aplicacion"
	suministroinfra "github.com/jairoprogramador/vex-engine/internal/suministro/infraestructura"
)

// El ACL sobre el Suministro de verdad, con el pipeline de ejemplo en un repositorio git temporal: lo que se
// comprueba es que trae la fuente, la convierte, la comprueba y siempre retira el material.

func nuevosPipelines(t *testing.T) (*infraestructura.PipelinesDeSuministro, string) {
	t.Helper()
	base := t.TempDir()
	s := suministro.NuevoServicio(suministro.Dependencias{
		Repositorios: suministroinfra.NuevosRepositoriosLocales(base),
		Hashes:       suministroinfra.HashDeContenido{},
	})
	return infraestructura.NuevosPipelinesDeSuministro(s), base
}

type fuente struct {
	t    *testing.T
	dir  string
	repo *git.Repository
}

// fuenteDelEjemplo es un repositorio con el pipeline de ejemplo, todavía sin commit.
func fuenteDelEjemplo(t *testing.T) *fuente {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	require.NoError(t, os.CopyFS(dir, os.DirFS(filepath.Join("..", "testdata", "ejemplo"))))
	return &fuente{t: t, dir: dir, repo: repo}
}

func (f *fuente) escribir(ruta, contenido string) *fuente {
	f.t.Helper()
	camino := filepath.Join(f.dir, filepath.FromSlash(ruta))
	require.NoError(f.t, os.MkdirAll(filepath.Dir(camino), 0o755))
	require.NoError(f.t, os.WriteFile(camino, []byte(contenido), 0o644))
	return f
}

func (f *fuente) commitear() string {
	f.t.Helper()
	w, err := f.repo.Worktree()
	require.NoError(f.t, err)
	require.NoError(f.t, w.AddWithOptions(&git.AddOptions{All: true}))
	id, err := w.Commit("cambio", &git.CommitOptions{
		Author: &object.Signature{Name: "ana", Email: "ana@vex.test", When: time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)},
	})
	require.NoError(f.t, err)
	return id.String()
}

func vacio(t *testing.T, base string) {
	t.Helper()
	entradas, err := os.ReadDir(base)
	require.NoError(t, err)
	require.Empty(t, entradas, "el material se retira")
}

func TestElPipelineDeHoyYElDeUnCommit(t *testing.T) {
	ctx := context.Background()
	pipelines, base := nuevosPipelines(t)
	f := fuenteDelEjemplo(t)
	primero := f.commitear()
	segundo := f.escribir("steps/01-pruebas/commands.yaml", "- name: verificar\n  cmd: mvn verify\n").commitear()

	hoy, err := pipelines.DeHoy(ctx, f.dir)
	require.NoError(t, err)
	require.Equal(t, segundo, hoy.Commit())
	require.NotEmpty(t, hoy.Hash())
	require.Len(t, hoy.Pasos(), 6)
	pruebas, _ := hoy.Paso("pruebas")
	require.Equal(t, "mvn verify", pruebas.Comandos[0].Linea)
	vacio(t, base)

	t.Run("el material conserva el bit de ejecución a través del repositorio", func(t *testing.T) {
		imagen, _ := hoy.Paso("imagen")
		ejecutables := map[string]bool{}
		for _, f := range imagen.Material {
			ejecutables[f.Ruta] = f.Ejecutable
		}
		require.Equal(t, map[string]bool{"docker/Dockerfile": false, "scripts/esperar.sh": true}, ejecutables)
	})

	antes, err := pipelines.DeUnCommit(ctx, f.dir, primero)
	require.NoError(t, err)
	require.Equal(t, primero, antes.Commit())
	require.NotEqual(t, hoy.Hash(), antes.Hash())
	pruebas, _ = antes.Paso("pruebas")
	require.Equal(t, "mvn clean verify", pruebas.Comandos[0].Linea, "un rollback lee el pipeline de su commit")
	vacio(t, base)
}

func TestElPipelineDeUnaCopiaDeTrabajo(t *testing.T) {
	ctx := context.Background()
	pipelines, base := nuevosPipelines(t)
	copia := t.TempDir()
	require.NoError(t, os.CopyFS(copia, os.DirFS(filepath.Join("..", "testdata", "ejemplo"))))

	p, err := pipelines.DeUnaCopiaDeTrabajo(ctx, copia)
	require.NoError(t, err)
	require.Empty(t, p.Commit(), "una copia de trabajo no tiene commit")
	require.NotEmpty(t, p.Hash())
	require.Len(t, p.Pasos(), 6)
	vacio(t, base)
}

func TestUnPipelineQueNoPasaLaComprobacionTambienSeRetira(t *testing.T) {
	pipelines, base := nuevosPipelines(t)
	f := fuenteDelEjemplo(t)
	f.escribir("config.yaml", "schema_version: 2\n").commitear()

	p, err := pipelines.DeHoy(context.Background(), f.dir)
	require.Nil(t, p)
	require.ErrorIs(t, err, dominio.ErrNoComprobado)
	require.Contains(t, err.Error(), `schema_version "2" no se lee`)
	vacio(t, base)
}

func TestLoQueNoEstaYLoQueNoSePuedePedir(t *testing.T) {
	ctx := context.Background()
	pipelines, _ := nuevosPipelines(t)
	f := fuenteDelEjemplo(t)
	f.commitear()

	_, err := pipelines.DeUnCommit(ctx, f.dir, strings.Repeat("b", 40))
	require.ErrorIs(t, err, dominio.ErrNoExiste)

	_, err = pipelines.DeUnCommit(ctx, f.dir, "main")
	require.ErrorIs(t, err, dominio.ErrInvalido)

	_, err = pipelines.DeHoy(ctx, filepath.Join(t.TempDir(), "no-esta"))
	require.ErrorIs(t, err, dominio.ErrNoExiste)
}
