package aplicacion_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/suministro/aplicacion"
	"github.com/jairoprogramador/vex-engine/internal/suministro/dominio"
	"github.com/jairoprogramador/vex-engine/internal/suministro/infraestructura"
	"github.com/jairoprogramador/vex-engine/internal/suministro/publicado"
)

// Los escenarios de RD-03 §7. Van sobre el acceso local y el hash de verdad, en directorios temporales: lo que
// se comprueba es justo lo que hacen ellos, y un adaptador en memoria no probaría nada. go-git trabaja dentro
// del proceso, así que no hace falta git instalado.

var cuando = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

func nuevoSuministro(t *testing.T) (*aplicacion.Servicio, string) {
	t.Helper()
	base := t.TempDir()
	return aplicacion.NuevoServicio(aplicacion.Dependencias{
		Repositorios: infraestructura.NuevosRepositoriosLocales(base),
		Hashes:       infraestructura.HashDeContenido{},
	}), base
}

type fuente struct {
	t    *testing.T
	dir  string
	repo *git.Repository
}

func nuevaFuente(t *testing.T) *fuente {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	return &fuente{t: t, dir: dir, repo: repo}
}

func (f *fuente) escribir(ruta, contenido string) *fuente {
	f.t.Helper()
	camino := filepath.Join(f.dir, filepath.FromSlash(ruta))
	require.NoError(f.t, os.MkdirAll(filepath.Dir(camino), 0o755))
	require.NoError(f.t, os.WriteFile(camino, []byte(contenido), 0o644))
	return f
}

func (f *fuente) commitear(mensaje string) string {
	f.t.Helper()
	w, err := f.repo.Worktree()
	require.NoError(f.t, err)
	require.NoError(f.t, w.AddWithOptions(&git.AddOptions{All: true}))
	id, err := w.Commit(mensaje, &git.CommitOptions{
		Author: &object.Signature{Name: "ana", Email: "ana@vex.test", When: cuando},
	})
	require.NoError(f.t, err)
	return id.String()
}

func leer(t *testing.T, material publicado.Material, ruta string) string {
	t.Helper()
	contenido, err := os.ReadFile(filepath.Join(material.Directorio, filepath.FromSlash(ruta)))
	require.NoError(t, err)
	return string(contenido)
}

func sinMateriales(t *testing.T, base string) {
	t.Helper()
	entradas, err := os.ReadDir(base)
	require.NoError(t, err)
	require.Empty(t, entradas)
}

func TestElMismoContenidoDaElMismoHash(t *testing.T) {
	ctx := context.Background()
	s, _ := nuevoSuministro(t)
	una := nuevaFuente(t).escribir("pom.xml", "<project/>").escribir("src/App.java", "class App {}")
	una.commitear("uno")
	otra := nuevaFuente(t).escribir("src/App.java", "class App {}").escribir("pom.xml", "<project/>")
	otra.commitear("otro")

	deUna, err := s.TraerDeHoy(ctx, una.dir)
	require.NoError(t, err)
	deOtra, err := s.TraerDeHoy(ctx, otra.dir)
	require.NoError(t, err)
	copia, err := s.TraerCopiaDeTrabajo(ctx, otra.dir)
	require.NoError(t, err)

	require.NotEqual(t, deUna.Commit, deOtra.Commit)
	require.Equal(t, deUna.Hash, deOtra.Hash)
	require.Equal(t, deUna.Hash, copia.Hash, "venga de un commit o de una copia de trabajo")
}

func TestUnCambioDeContenidoCambiaElHash(t *testing.T) {
	ctx := context.Background()
	s, _ := nuevoSuministro(t)
	f := nuevaFuente(t).escribir("pom.xml", "<project/>")
	f.commitear("uno")
	antes, err := s.TraerDeHoy(ctx, f.dir)
	require.NoError(t, err)

	f.escribir("pom.xml", "<project><version>2</version></project>")
	f.commitear("dos")
	despues, err := s.TraerDeHoy(ctx, f.dir)
	require.NoError(t, err)

	require.NotEqual(t, antes.Hash, despues.Hash)
}

func TestSiSeRevierteUnCambioElCommitEsNuevoYElHashElDeAntes(t *testing.T) {
	ctx := context.Background()
	s, _ := nuevoSuministro(t)
	f := nuevaFuente(t).escribir("deploy.sh", "kubectl apply")
	primero := f.commitear("uno")
	original, err := s.TraerDeHoy(ctx, f.dir)
	require.NoError(t, err)

	f.escribir("deploy.sh", "kubectl delete")
	cambio := f.commitear("cambio")
	cambiado, err := s.TraerDeHoy(ctx, f.dir)
	require.NoError(t, err)
	require.Equal(t, cambio, cambiado.Commit)
	require.NotEqual(t, original.Hash, cambiado.Hash)

	f.escribir("deploy.sh", "kubectl apply")
	revertido := f.commitear("revertir el cambio")
	despues, err := s.TraerDeHoy(ctx, f.dir)
	require.NoError(t, err)
	require.Equal(t, revertido, despues.Commit)
	require.NotEqual(t, primero, despues.Commit, "el commit es nuevo")
	require.Equal(t, original.Hash, despues.Hash, "el hash es el de antes")

	deEntonces, err := s.TraerDeUnCommit(ctx, f.dir, cambio)
	require.NoError(t, err)
	require.Equal(t, cambio, deEntonces.Commit)
	require.Equal(t, cambiado.Hash, deEntonces.Hash)
	require.Equal(t, "kubectl delete", leer(t, deEntonces, "deploy.sh"), "el material tal como era")
}

func TestUnaCopiaDeTrabajoTieneHashYNoTieneCommit(t *testing.T) {
	ctx := context.Background()
	s, _ := nuevoSuministro(t)
	f := nuevaFuente(t).escribir(".gitignore", "target/\n").escribir("pom.xml", "<project/>")
	f.commitear("uno")
	deHoy, err := s.TraerDeHoy(ctx, f.dir)
	require.NoError(t, err)

	f.escribir("target/app.jar", "lo genera la tecnología")
	sinCambios, err := s.TraerCopiaDeTrabajo(ctx, f.dir)
	require.NoError(t, err)
	require.Empty(t, sinCambios.Commit)
	require.NotEmpty(t, sinCambios.Hash)
	require.Equal(t, deHoy.Hash, sinCambios.Hash, "sin cambios, el hash es el de su commit")

	f.escribir("pom.xml", "<project><version>2</version></project>")
	conCambios, err := s.TraerCopiaDeTrabajo(ctx, f.dir)
	require.NoError(t, err)
	require.Empty(t, conCambios.Commit)
	require.NotEqual(t, deHoy.Hash, conCambios.Hash)

	hoyTodavia, err := s.TraerDeHoy(ctx, f.dir)
	require.NoError(t, err)
	require.Equal(t, deHoy.Commit, hoyTodavia.Commit, "lo que no tiene commit no está hoy")
	require.Equal(t, deHoy.Hash, hoyTodavia.Hash)
}

func TestLoQueNoSePuedePedirYLoQueNoEsta(t *testing.T) {
	ctx := context.Background()
	s, base := nuevoSuministro(t)
	conCommit := nuevaFuente(t).escribir("a", "a")
	conCommit.commitear("uno")
	sinCommits := nuevaFuente(t)
	noRepositorio := t.TempDir()

	casos := []struct {
		nombre string
		traer  func() (publicado.Material, error)
		err    error
	}{
		{"una fuente sin ubicación", func() (publicado.Material, error) {
			return s.TraerDeHoy(ctx, "")
		}, publicado.ErrInvalido},
		{"una copia de trabajo sin directorio", func() (publicado.Material, error) {
			return s.TraerCopiaDeTrabajo(ctx, "")
		}, publicado.ErrInvalido},
		{"un commit abreviado", func() (publicado.Material, error) {
			return s.TraerDeUnCommit(ctx, conCommit.dir, "abc1234")
		}, publicado.ErrInvalido},
		{"una rama en vez de un commit", func() (publicado.Material, error) {
			return s.TraerDeUnCommit(ctx, conCommit.dir, "master")
		}, publicado.ErrInvalido},
		{"un commit que la fuente no tiene", func() (publicado.Material, error) {
			return s.TraerDeUnCommit(ctx, conCommit.dir, strings.Repeat("0", 40))
		}, publicado.ErrNoExiste},
		{"una fuente que no es un repositorio", func() (publicado.Material, error) {
			return s.TraerDeHoy(ctx, noRepositorio)
		}, publicado.ErrNoExiste},
		{"una fuente sin commits", func() (publicado.Material, error) {
			return s.TraerDeHoy(ctx, sinCommits.dir)
		}, publicado.ErrNoExiste},
		{"una copia de trabajo que no está", func() (publicado.Material, error) {
			return s.TraerCopiaDeTrabajo(ctx, filepath.Join(noRepositorio, "no"))
		}, publicado.ErrNoExiste},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := c.traer()
			require.ErrorIs(t, err, c.err)
		})
	}
	sinMateriales(t, base)
}

func TestRetirarBorraElMaterialYNadaMas(t *testing.T) {
	ctx := context.Background()
	s, base := nuevoSuministro(t)
	f := nuevaFuente(t).escribir("values.yaml", "replicas: 1")
	f.commitear("uno")
	material, err := s.TraerDeHoy(ctx, f.dir)
	require.NoError(t, err)

	require.ErrorIs(t, s.Retirar(ctx, publicado.Material{Directorio: f.dir}), publicado.ErrInvalido)
	require.ErrorIs(t, s.Retirar(ctx, publicado.Material{}), publicado.ErrInvalido)
	require.FileExists(t, filepath.Join(f.dir, "values.yaml"))

	require.NoError(t, s.Retirar(ctx, material))
	sinMateriales(t, base)
}

type hashesQueFallan struct{}

func (hashesQueFallan) DeUnDirectorio(context.Context, string) (dominio.Hash, error) {
	return dominio.Hash{}, errors.New("disco roto")
}

func TestSinHashNoSeEntregaMaterial(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	s := aplicacion.NuevoServicio(aplicacion.Dependencias{
		Repositorios: infraestructura.NuevosRepositoriosLocales(base),
		Hashes:       hashesQueFallan{},
	})
	f := nuevaFuente(t).escribir("values.yaml", "replicas: 1")
	f.commitear("uno")

	_, err := s.TraerDeHoy(ctx, f.dir)
	require.ErrorContains(t, err, "disco roto")
	sinMateriales(t, base)
}
