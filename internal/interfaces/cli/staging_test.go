package cli

// Tests del área de trabajo del motor (spec 16 §5.3). Van en `package cli` y no
// en `cli_test` porque lo que se prueba es la CADENA DE RESOLUCIÓN, que no tiene
// —ni debe tener— superficie pública: desde fuera sólo se ve el directorio ya
// resuelto.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aislarElEntorno deja las tres fuentes de la cadena bajo control del test: sin
// esto, un XDG_STATE_HOME real haría pasar o fallar los casos según la máquina.
func aislarElEntorno(t *testing.T) (xdg, home string) {
	t.Helper()
	xdg = filepath.Join(t.TempDir(), "xdg")
	home = filepath.Join(t.TempDir(), "home")
	require.NoError(t, os.MkdirAll(xdg, 0o755))
	require.NoError(t, os.MkdirAll(home, 0o755))
	t.Setenv(xdgStateHomeEnv, xdg)
	t.Setenv("HOME", home)
	return xdg, home
}

func TestResolveStagingDir_LaFlagGanaYSeCrea(t *testing.T) {
	xdg, _ := aislarElEntorno(t)
	pedido := filepath.Join(t.TempDir(), "mi-staging")

	dir, err := resolveStagingDir(pedido, filepath.Join(t.TempDir(), "volumen"))

	require.NoError(t, err)
	assert.Equal(t, pedido, dir)
	assert.DirExists(t, dir, "el motor es dueño de su área de trabajo: la crea, no la pide prestada")
	assert.NoDirExists(t, filepath.Join(xdg, stagingVendorDir))
}

// La cadena completa, en orden, y el fallback que la §5.3 existe para arreglar:
// `/var/lib/vex/…` devolvía EACCES al primer arranque porque el contenedor corre
// como uid 1001 y `/var/lib` es de root, así que la escritura «incondicional»
// dejaba de serlo. Cada candidato se PRUEBA; el que no sirve se salta.
func TestResolveStagingDir_LaCadenaDeResolucion(t *testing.T) {
	volumen := filepath.Join(t.TempDir(), "vol")

	t.Run("sin flag manda XDG_STATE_HOME", func(t *testing.T) {
		xdg, home := aislarElEntorno(t)

		dir, err := resolveStagingDir("", volumen)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(xdg, stagingVendorDir, stagingLeafDir), dir)
		assert.NoDirExists(t, filepath.Join(home, ".local"))
	})

	t.Run("XDG inescribible cae a $HOME/.local/state", func(t *testing.T) {
		siEsRoot(t)
		xdg, home := aislarElEntorno(t)
		require.NoError(t, os.Chmod(xdg, 0o555))
		t.Cleanup(func() { _ = os.Chmod(xdg, 0o755) })

		dir, err := resolveStagingDir("", volumen)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".local", "state", stagingVendorDir, stagingLeafDir), dir)
		assert.DirExists(t, dir)
	})

	t.Run("con las dos inescribibles cae a TempDir", func(t *testing.T) {
		siEsRoot(t)
		xdg, home := aislarElEntorno(t)
		require.NoError(t, os.Chmod(xdg, 0o555))
		require.NoError(t, os.Chmod(home, 0o555))
		t.Cleanup(func() {
			_ = os.Chmod(xdg, 0o755)
			_ = os.Chmod(home, 0o755)
		})

		dir, err := resolveStagingDir("", volumen)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(os.TempDir(), stagingVendorDir, stagingLeafDir), dir)
		assert.DirExists(t, dir)
	})

	t.Run("un directorio que ya existe sin permiso de escritura no cuenta", func(t *testing.T) {
		// `MkdirAll` sobre un directorio existente no falla, así que sin la
		// prueba de escritura este candidato se daría por bueno y el fallo
		// aparecería a mitad de la ejecución, con el primer registro.
		siEsRoot(t)
		xdg, home := aislarElEntorno(t)
		ocupado := filepath.Join(xdg, stagingVendorDir, stagingLeafDir)
		require.NoError(t, os.MkdirAll(ocupado, 0o755))
		require.NoError(t, os.Chmod(ocupado, 0o555))
		t.Cleanup(func() { _ = os.Chmod(ocupado, 0o755) })

		dir, err := resolveStagingDir("", volumen)

		require.NoError(t, err)
		assert.Equal(t, filepath.Join(home, ".local", "state", stagingVendorDir, stagingLeafDir), dir)
	})
}

// LA ASERCIÓN EXPLÍCITA de §5.3: el área de trabajo NUNCA cae bajo el volumen.
// Si cayera, un `type: local` mal configurado dejaría de ser detectable —el motor
// estaría escribiendo su búfer dentro del sitio al que cree estar sincronizando—
// y la spec 21 no tendría desde dónde re-empujar.
func TestResolveStagingDir_NuncaBajoElVolumen(t *testing.T) {
	rootVexPath := t.TempDir()
	volumen := filepath.Join(rootVexPath, VexHomeDirName)

	t.Run("un candidato de la cadena que cae dentro se salta", func(t *testing.T) {
		_, home := aislarElEntorno(t)
		t.Setenv(xdgStateHomeEnv, volumen)

		dir, err := resolveStagingDir("", rootVexPath)

		require.NoError(t, err)
		assert.False(t, estaBajo(dir, volumen))
		assert.Equal(t, filepath.Join(home, ".local", "state", stagingVendorDir, stagingLeafDir), dir)
	})

	t.Run("la flag que cae dentro es un error, no una sustitución silenciosa", func(t *testing.T) {
		aislarElEntorno(t)

		_, err := resolveStagingDir(filepath.Join(volumen, "staging"), rootVexPath)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInputInvalido)
		assert.Contains(t, err.Error(), "--staging-dir")
	})
}

func TestEstaBajo_NoSeDejaEnganarPorElPrefijo(t *testing.T) {
	raiz := filepath.Join(string(filepath.Separator), "vexHome")

	assert.True(t, estaBajo(raiz, raiz))
	assert.True(t, estaBajo(filepath.Join(raiz, "state"), raiz))
	assert.False(t, estaBajo(raiz+"-otro", raiz),
		"un vecino con el mismo prefijo no está dentro")
}

func siEsRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("como root los permisos no impiden escribir, y el caso mide justo eso")
	}
}
