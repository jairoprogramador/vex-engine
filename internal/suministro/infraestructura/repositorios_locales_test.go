package infraestructura_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/suministro/dominio"
	"github.com/jairoprogramador/vex-engine/internal/suministro/infraestructura"
)

func fuente(t *testing.T, dir string) dominio.Fuente {
	t.Helper()
	f, err := dominio.NuevaFuente(dir)
	require.NoError(t, err)
	return f
}

func copiaDeTrabajo(t *testing.T, dir string) dominio.CopiaDeTrabajo {
	t.Helper()
	c, err := dominio.NuevaCopiaDeTrabajo(dir)
	require.NoError(t, err)
	return c
}

func commitDe(t *testing.T, id string) dominio.Commit {
	t.Helper()
	c, err := dominio.NuevoCommit(id)
	require.NoError(t, err)
	return c
}

func TestDeHoyEsElArbolDelUltimoCommit(t *testing.T) {
	ctx := context.Background()
	dir, repo := repositorio(t)
	escribir(t, dir, "pom.xml", "<project/>")
	escribir(t, dir, "bin/deploy.sh", "kubectl apply")
	require.NoError(t, os.Chmod(filepath.Join(dir, "bin/deploy.sh"), 0o755))
	require.NoError(t, os.Symlink("bin/deploy.sh", filepath.Join(dir, "deploy")))
	id := commitear(t, repo, "uno")

	escribir(t, dir, "pom.xml", "<project><version>2</version></project>")
	escribir(t, dir, "sin-commit.txt", "todavía no")

	r := infraestructura.NuevosRepositoriosLocales(t.TempDir())
	material, commit, err := r.PonerDeHoy(ctx, fuente(t, dir))
	require.NoError(t, err)

	require.Equal(t, id, commit.String())
	require.Equal(t, []string{"bin/deploy.sh", "deploy", "pom.xml"}, rutas(t, material))
	require.Equal(t, "<project/>", leer(t, material, "pom.xml"))
	info, err := os.Stat(filepath.Join(material, "bin/deploy.sh"))
	require.NoError(t, err)
	require.NotZero(t, info.Mode().Perm()&0o111, "un ejecutable sigue siéndolo")
	destino, err := os.Readlink(filepath.Join(material, "deploy"))
	require.NoError(t, err)
	require.Equal(t, "bin/deploy.sh", destino)
}

func TestDeUnCommitEsElArbolDeEseCommit(t *testing.T) {
	ctx := context.Background()
	dir, repo := repositorio(t)
	escribir(t, dir, "values.yaml", "replicas: 1")
	primero := commitear(t, repo, "uno")
	escribir(t, dir, "values.yaml", "replicas: 3")
	escribir(t, dir, "nuevo.yaml", "")
	commitear(t, repo, "dos")

	r := infraestructura.NuevosRepositoriosLocales(t.TempDir())
	material, err := r.PonerDeUnCommit(ctx, fuente(t, dir), commitDe(t, primero))
	require.NoError(t, err)

	require.Equal(t, []string{"values.yaml"}, rutas(t, material))
	require.Equal(t, "replicas: 1", leer(t, material, "values.yaml"))
}

func TestLoQueNoEstaEnLosRepositorios(t *testing.T) {
	ctx := context.Background()
	r := infraestructura.NuevosRepositoriosLocales(t.TempDir())
	conCommit, repo := repositorio(t)
	escribir(t, conCommit, "a", "a")
	commitear(t, repo, "uno")
	sinCommits, _ := repositorio(t)
	noRepositorio := t.TempDir()

	_, _, err := r.PonerDeHoy(ctx, fuente(t, noRepositorio))
	require.ErrorIs(t, err, dominio.ErrNoExiste)

	_, _, err = r.PonerDeHoy(ctx, fuente(t, sinCommits))
	require.ErrorIs(t, err, dominio.ErrNoExiste)

	_, err = r.PonerDeUnCommit(ctx, fuente(t, conCommit), commitDe(t, strings.Repeat("0", 40)))
	require.ErrorIs(t, err, dominio.ErrNoExiste)

	_, err = r.PonerCopiaDeTrabajo(ctx, copiaDeTrabajo(t, filepath.Join(noRepositorio, "no")))
	require.ErrorIs(t, err, dominio.ErrNoExiste)
}

func TestLaCopiaDeTrabajoEsLoQueEntrariaEnUnCommit(t *testing.T) {
	ctx := context.Background()
	dir, repo := repositorio(t)
	// Un fichero que se sigue desde antes de que su directorio se ignore.
	escribir(t, dir, "build/conservar.txt", "seguido")
	commitear(t, repo, "uno")

	escribir(t, dir, ".gitignore", "build/\n*.log\n!importante.log\n")
	escribir(t, dir, "build/conservar.txt", "seguido y cambiado")
	escribir(t, dir, "build/app.jar", "binario")
	escribir(t, dir, "traza.log", "ruido")
	escribir(t, dir, "importante.log", "se queda")
	escribir(t, dir, "src/.gitignore", "generado.txt\n")
	escribir(t, dir, "src/App.java", "class App {}")
	escribir(t, dir, "src/generado.txt", "ruido")
	escribir(t, dir, "target/classes/App.class", "sin ignorar, entra")
	anidado, err := git.PlainInit(filepath.Join(dir, "vendor", "otro"), false)
	require.NoError(t, err)
	require.NotNil(t, anidado)
	escribir(t, dir, "vendor/otro/lib.go", "package otro")

	r := infraestructura.NuevosRepositoriosLocales(t.TempDir())
	material, err := r.PonerCopiaDeTrabajo(ctx, copiaDeTrabajo(t, dir))
	require.NoError(t, err)

	require.Equal(t, []string{
		".gitignore",
		"build/conservar.txt",
		"importante.log",
		"src/.gitignore",
		"src/App.java",
		"target/classes/App.class",
	}, rutas(t, material))
	require.Equal(t, "seguido y cambiado", leer(t, material, "build/conservar.txt"))
}

func TestUnaCopiaDeTrabajoQueNoEsRepositorioSoloUsaSusGitignore(t *testing.T) {
	dir := t.TempDir()
	escribir(t, dir, ".gitignore", "node_modules/\n")
	escribir(t, dir, "package.json", "{}")
	escribir(t, dir, "node_modules/x/index.js", "")

	r := infraestructura.NuevosRepositoriosLocales(t.TempDir())
	material, err := r.PonerCopiaDeTrabajo(context.Background(), copiaDeTrabajo(t, dir))
	require.NoError(t, err)

	require.Equal(t, []string{".gitignore", "package.json"}, rutas(t, material))
}

func TestElMaterialDeUnaCopiaDeTrabajoNoCambiaConElla(t *testing.T) {
	dir := t.TempDir()
	escribir(t, dir, "values.yaml", "replicas: 1")

	r := infraestructura.NuevosRepositoriosLocales(t.TempDir())
	material, err := r.PonerCopiaDeTrabajo(context.Background(), copiaDeTrabajo(t, dir))
	require.NoError(t, err)
	escribir(t, dir, "values.yaml", "replicas: 3")

	require.Equal(t, "replicas: 1", leer(t, material, "values.yaml"))
}

func TestRetirarSoloBorraLoQuePuso(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	r := infraestructura.NuevosRepositoriosLocales(base)
	dir := t.TempDir()
	escribir(t, dir, "values.yaml", "replicas: 1")
	material, err := r.PonerCopiaDeTrabajo(ctx, copiaDeTrabajo(t, dir))
	require.NoError(t, err)

	require.ErrorIs(t, r.Retirar(ctx, dir), dominio.ErrInvalido, "la copia de trabajo no es un material")
	require.ErrorIs(t, r.Retirar(ctx, base), dominio.ErrInvalido)
	require.FileExists(t, filepath.Join(dir, "values.yaml"))

	require.NoError(t, r.Retirar(ctx, material))
	require.NoDirExists(t, material)
}
