package infraestructura_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
)

func TestDirectorioEfimero_SeCreaVacioYSoloParaElUsuario(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "efimero")
	efimero := infraestructura.NuevoDirectorioEfimero(ruta)

	creado, err := efimero.Crear(context.Background())

	require.NoError(t, err)
	require.Equal(t, ruta, creado, "la ruta es la que se dijo: no cambia de un paso a otro")
	info, err := os.Lstat(ruta)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
	}
	entradas, err := os.ReadDir(ruta)
	require.NoError(t, err)
	require.Empty(t, entradas)
}

func TestDirectorioEfimero_CrearNoHeredaLoDelPasoAnterior(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "efimero")
	efimero := infraestructura.NuevoDirectorioEfimero(ruta)
	_, err := efimero.Crear(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(ruta, "azure"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(ruta, "azure", "msal_token_cache.json"), []byte("sesión"), 0o600))

	_, err = efimero.Crear(context.Background())

	require.NoError(t, err)
	entradas, err := os.ReadDir(ruta)
	require.NoError(t, err)
	require.Empty(t, entradas, "aunque el paso anterior no llegara a borrarlo")
}

func TestDirectorioEfimero_BorrarSeLoLlevaTodoYSePuedeRepetir(t *testing.T) {
	ruta := filepath.Join(t.TempDir(), "efimero")
	efimero := infraestructura.NuevoDirectorioEfimero(ruta)
	_, err := efimero.Crear(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(ruta, "sesion"), []byte("x"), 0o600))

	require.NoError(t, efimero.Borrar(context.Background()))

	_, err = os.Lstat(ruta)
	require.True(t, os.IsNotExist(err))
	require.NoError(t, efimero.Borrar(context.Background()), "borrar lo que no está no es un error")
}

func TestDirectorioEfimero_UnEnlacePuestoEnSuSitioNoSeSigue(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("crear enlaces simbólicos en Windows pide privilegios")
	}
	base := t.TempDir()
	ajeno := filepath.Join(base, "ajeno")
	require.NoError(t, os.Mkdir(ajeno, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ajeno, "dato"), []byte("no se toca"), 0o644))
	ruta := filepath.Join(base, "efimero")
	require.NoError(t, os.Symlink(ajeno, ruta))
	efimero := infraestructura.NuevoDirectorioEfimero(ruta)

	_, err := efimero.Crear(context.Background())

	require.NoError(t, err)
	info, err := os.Lstat(ruta)
	require.NoError(t, err)
	require.True(t, info.IsDir(), "es un directorio de verdad, no el enlace")
	contenido, err := os.ReadFile(filepath.Join(ajeno, "dato"))
	require.NoError(t, err, "a lo que apuntaba el enlace no se le borra nada")
	require.Equal(t, "no se toca", string(contenido))
	entradas, err := os.ReadDir(ajeno)
	require.NoError(t, err)
	require.Len(t, entradas, 1, "ni se le escribe nada")
}

func TestRutaDelDirectorioEfimero_ConBaseEsFijaYConocida(t *testing.T) {
	require.Equal(t, filepath.Join("/tmp", "vex-step-tmp"), infraestructura.RutaDelDirectorioEfimero("/tmp"))
}

func TestRutaDelDirectorioEfimero_SinBaseEsDeEsteProceso(t *testing.T) {
	esperada := filepath.Join(os.TempDir(), "vex-step-tmp-"+strconv.Itoa(os.Getpid()))
	require.Equal(t, esperada, infraestructura.RutaDelDirectorioEfimero(""))
}

func TestDirectorioEfimero_CreaLaBaseSiNoEsta(t *testing.T) {
	ruta := infraestructura.RutaDelDirectorioEfimero(filepath.Join(t.TempDir(), "aun", "no"))

	_, err := infraestructura.NuevoDirectorioEfimero(ruta).Crear(context.Background())

	require.NoError(t, err)
	require.DirExists(t, ruta)
}
