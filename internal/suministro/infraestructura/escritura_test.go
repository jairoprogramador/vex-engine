package infraestructura

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLaEscrituraNoSaleDelMaterial(t *testing.T) {
	casos := []struct {
		nombre string
		ruta   string
	}{
		{"subiendo", "../fuera.txt"},
		{"subiendo desde dentro", "a/../../fuera.txt"},
		{"absoluta", "/tmp/fuera.txt"},
		{"a un .git", ".git/config"},
		{"a un .git en mayúsculas", "src/.GIT/hooks/pre-commit"},
		{"a través de un enlace", "enlace/fuera.txt"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			raiz := t.TempDir()
			fuera := t.TempDir()
			require.NoError(t, os.Symlink(fuera, filepath.Join(raiz, "enlace")))
			e := &escritura{raiz: raiz}

			require.Error(t, e.fichero(c.ruta, false, strings.NewReader("x")))
			entradas, err := os.ReadDir(fuera)
			require.NoError(t, err)
			require.Empty(t, entradas)
		})
	}
}
