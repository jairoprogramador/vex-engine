package infraestructura_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/suministro/infraestructura"
)

func hashDe(t *testing.T, directorio string) string {
	t.Helper()
	h, err := infraestructura.HashDeContenido{}.DeUnDirectorio(context.Background(), directorio)
	require.NoError(t, err)
	return h.String()
}

// base es el contenido del que parten las pruebas del hash.
func base(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	escribir(t, dir, "pom.xml", "<project/>")
	escribir(t, dir, "src/App.java", "class App {}")
	escribir(t, dir, "deploy.sh", "kubectl apply")
	require.NoError(t, os.Symlink("deploy.sh", filepath.Join(dir, "actual")))
	return dir
}

func TestElMismoContenidoDaElMismoHash(t *testing.T) {
	una := base(t)

	otra := t.TempDir()
	require.NoError(t, os.Symlink("deploy.sh", filepath.Join(otra, "actual")))
	escribir(t, otra, "deploy.sh", "kubectl apply")
	escribir(t, otra, "src/App.java", "class App {}")
	escribir(t, otra, "pom.xml", "<project/>")
	// Ni las fechas ni un directorio vacío son contenido.
	require.NoError(t, os.Chtimes(filepath.Join(otra, "pom.xml"), cuando, cuando.Add(time.Hour)))
	require.NoError(t, os.Mkdir(filepath.Join(otra, "vacio"), 0o755))

	require.Equal(t, hashDe(t, una), hashDe(t, otra))
	require.True(t, strings.HasPrefix(hashDe(t, una), "contenido-v1:"))
}

func TestUnCambioDeContenidoCambiaElHash(t *testing.T) {
	casos := []struct {
		nombre  string
		cambiar func(t *testing.T, dir string)
	}{
		{"el contenido de un fichero", func(t *testing.T, dir string) {
			escribir(t, dir, "pom.xml", "<project><version>2</version></project>")
		}},
		{"un fichero nuevo", func(t *testing.T, dir string) {
			escribir(t, dir, "README.md", "")
		}},
		{"un fichero borrado", func(t *testing.T, dir string) {
			require.NoError(t, os.Remove(filepath.Join(dir, "pom.xml")))
		}},
		{"el nombre de un fichero", func(t *testing.T, dir string) {
			require.NoError(t, os.Rename(filepath.Join(dir, "pom.xml"), filepath.Join(dir, "build.xml")))
		}},
		{"un fichero que pasa a otro directorio", func(t *testing.T, dir string) {
			require.NoError(t, os.Rename(filepath.Join(dir, "pom.xml"), filepath.Join(dir, "src", "pom.xml")))
		}},
		{"el bit de ejecución", func(t *testing.T, dir string) {
			require.NoError(t, os.Chmod(filepath.Join(dir, "deploy.sh"), 0o755))
		}},
		{"el destino de un enlace", func(t *testing.T, dir string) {
			require.NoError(t, os.Remove(filepath.Join(dir, "actual")))
			require.NoError(t, os.Symlink("pom.xml", filepath.Join(dir, "actual")))
		}},
		{"un enlace que pasa a ser el fichero al que apuntaba", func(t *testing.T, dir string) {
			require.NoError(t, os.Remove(filepath.Join(dir, "actual")))
			escribir(t, dir, "actual", "deploy.sh")
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := base(t)
			antes := hashDe(t, dir)
			c.cambiar(t, dir)
			require.NotEqual(t, antes, hashDe(t, dir))
		})
	}
}

func TestSinDirectorioNoHayHash(t *testing.T) {
	_, err := infraestructura.HashDeContenido{}.DeUnDirectorio(context.Background(), filepath.Join(t.TempDir(), "no"))
	require.Error(t, err)
}
