package fingerprint_test

// El contrato del puerto TreeSource, comprobado sobre sus dos implementaciones.
//
// No es un test de conveniencia: la regla v1 depende del orden del recorrido
// —la precedencia de las reglas .gitignore es «gana la última que casa»— y de
// que Open no siga los enlaces. Una fuente que incumpla cualquiera de las dos
// cosas produce una huella distinta sin fallar en ninguna parte.

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	infraFingerprint "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/fingerprint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// El árbol de referencia, declarado una vez y materializado en las dos fuentes.
var arbolDeReferencia = []struct {
	path    string
	content string
	kind    string // "file" | "exec" | "dir" | "link"
}{
	{path: ".gitignore", content: "*.log\n", kind: "file"},
	{path: "a.txt", content: "contenido a", kind: "file"},
	{path: "bin/run.sh", content: "#!/bin/sh\n", kind: "exec"},
	{path: "sub/.gitignore", content: "!deep.log\n", kind: "file"},
	{path: "sub/b.txt", content: "contenido b", kind: "file"},
	{path: "sub/deep/c.txt", content: "contenido c", kind: "file"},
	{path: "vacio", kind: "dir"},
	{path: "zlink", content: "a.txt", kind: "link"},
}

func nuevaFuenteEnMemoria() *infraFingerprint.MemTreeSource {
	src := infraFingerprint.NewMemTreeSource()
	for _, n := range arbolDeReferencia {
		switch n.kind {
		case "file":
			src.AddFile(n.path, n.content)
		case "exec":
			src.AddExecutable(n.path, n.content)
		case "dir":
			src.AddDir(n.path)
		case "link":
			src.AddSymlink(n.path, n.content)
		}
	}
	return src
}

func nuevaFuenteEnDisco(t *testing.T) *infraFingerprint.DirTreeSource {
	t.Helper()
	root := t.TempDir()
	for _, n := range arbolDeReferencia {
		abs := filepath.Join(root, filepath.FromSlash(n.path))
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		switch n.kind {
		case "file":
			require.NoError(t, os.WriteFile(abs, []byte(n.content), 0o644))
		case "exec":
			require.NoError(t, os.WriteFile(abs, []byte(n.content), 0o755))
		case "dir":
			require.NoError(t, os.MkdirAll(abs, 0o755))
		case "link":
			require.NoError(t, os.Symlink(n.content, abs))
		}
	}

	src, err := infraFingerprint.NewDirTreeSource(root)
	require.NoError(t, err)
	return src
}

func fuentes(t *testing.T) map[string]domFingerprint.TreeSource {
	t.Helper()
	return map[string]domFingerprint.TreeSource{
		"memoria": nuevaFuenteEnMemoria(),
		"disco":   nuevaFuenteEnDisco(t),
	}
}

func TestTreeSource_ContratoDelRecorrido(t *testing.T) {
	for nombre, src := range fuentes(t) {
		t.Run(nombre, func(t *testing.T) {
			type visto struct {
				path      string
				isDir     bool
				isSymlink bool
				exec      bool
			}
			var rutas []visto

			require.NoError(t, src.Walk(func(path string, isDir, isSymlink bool, mode fs.FileMode) error {
				rutas = append(rutas, visto{path, isDir, isSymlink, mode&0o111 != 0})
				return nil
			}))

			// Orden: lexicográfico dentro de cada directorio, padre antes que
			// contenido, y la raíz no se emite.
			assert.Equal(t, []visto{
				{path: ".gitignore"},
				{path: "a.txt"},
				{path: "bin", isDir: true, exec: true},
				{path: "bin/run.sh", exec: true},
				{path: "sub", isDir: true, exec: true},
				{path: "sub/.gitignore"},
				{path: "sub/b.txt"},
				{path: "sub/deep", isDir: true, exec: true},
				{path: "sub/deep/c.txt"},
				{path: "vacio", isDir: true, exec: true},
				// Un enlace llega con el bit de ejecución puesto en las dos
				// fuentes, y no significa nada: los permisos que cuentan son
				// los del destino. Por eso la regla lo marca como enlace antes
				// de mirar el bit (SPEC-v1.md §3.2).
				{path: "zlink", isSymlink: true, exec: true},
			}, rutas)
		})
	}
}

// Un enlace nunca es un directorio, apunte a donde apunte: si lo fuera, el
// recorrido entraría y la identidad se saldría del árbol.
func TestTreeSource_UnEnlaceNoEsUnDirectorio(t *testing.T) {
	memoria := infraFingerprint.NewMemTreeSource().
		AddFile("sub/a.txt", "contenido a").
		AddSymlink("link", "sub")

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", "a.txt"), []byte("contenido a"), 0o644))
	require.NoError(t, os.Symlink("sub", filepath.Join(root, "link")))
	disco, err := infraFingerprint.NewDirTreeSource(root)
	require.NoError(t, err)

	for nombre, src := range map[string]domFingerprint.TreeSource{"memoria": memoria, "disco": disco} {
		t.Run(nombre, func(t *testing.T) {
			var rutas []string
			require.NoError(t, src.Walk(func(path string, isDir, isSymlink bool, _ fs.FileMode) error {
				if path == "link" {
					assert.False(t, isDir)
					assert.True(t, isSymlink)
				}
				rutas = append(rutas, path)
				return nil
			}))

			assert.Equal(t, []string{"link", "sub", "sub/a.txt"}, rutas)
		})
	}
}

func TestTreeSource_SkipDirOmiteElContenidoYNoALosHermanos(t *testing.T) {
	for nombre, src := range fuentes(t) {
		t.Run(nombre, func(t *testing.T) {
			var rutas []string

			require.NoError(t, src.Walk(func(path string, isDir, _ bool, _ fs.FileMode) error {
				if isDir && path == "sub" {
					return fs.SkipDir
				}
				rutas = append(rutas, path)
				return nil
			}))

			assert.Equal(t, []string{
				".gitignore", "a.txt", "bin", "bin/run.sh", "vacio", "zlink",
			}, rutas)
		})
	}
}

func TestTreeSource_ElErrorDelCallbackAbortaElRecorrido(t *testing.T) {
	for nombre, src := range fuentes(t) {
		t.Run(nombre, func(t *testing.T) {
			roto := io.ErrUnexpectedEOF
			visitadas := 0

			err := src.Walk(func(string, bool, bool, fs.FileMode) error {
				visitadas++
				return roto
			})

			require.ErrorIs(t, err, roto)
			assert.Equal(t, 1, visitadas)
		})
	}
}

func TestTreeSource_ContratoDeOpen(t *testing.T) {
	for nombre, src := range fuentes(t) {
		t.Run(nombre, func(t *testing.T) {
			t.Run("archivo regular: su contenido", func(t *testing.T) {
				assert.Equal(t, "contenido a", leer(t, src, "a.txt"))
			})

			t.Run("enlace: su destino, sin seguirlo", func(t *testing.T) {
				assert.Equal(t, "a.txt", leer(t, src, "zlink"))
			})

			t.Run("ruta inexistente: un error que envuelve fs.ErrNotExist", func(t *testing.T) {
				_, err := src.Open("no-existe.txt")
				require.Error(t, err)
				assert.ErrorIs(t, err, fs.ErrNotExist)
			})

			t.Run("directorio: un error", func(t *testing.T) {
				reader, err := src.Open("sub")
				if err == nil {
					_, err = io.ReadAll(reader)
					reader.Close()
				}
				require.Error(t, err)
			})
		})
	}
}

func leer(t *testing.T, src domFingerprint.TreeSource, path string) string {
	t.Helper()
	reader, err := src.Open(path)
	require.NoError(t, err)
	defer reader.Close()

	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	return string(content)
}

func TestNewDirTreeSource_RechazaLoQueNoEsUnDirectorio(t *testing.T) {
	root := t.TempDir()
	archivo := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(archivo, []byte("contenido a"), 0o644))

	t.Run("archivo", func(t *testing.T) {
		_, err := infraFingerprint.NewDirTreeSource(archivo)
		require.Error(t, err)
	})

	t.Run("ruta inexistente", func(t *testing.T) {
		_, err := infraFingerprint.NewDirTreeSource(filepath.Join(root, "no-existe"))
		require.Error(t, err)
	})
}
