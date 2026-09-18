package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestUnComandoDeclaradoNoPuedeTenerLaLineaVacia(t *testing.T) {
	_, err := dominio.NuevoComandoDeclarado("build", "", "", nil, nil, nil)
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnComandoDeclaradoGuardaLoQueDeclara(t *testing.T) {
	salida, err := dominio.NuevaVariableDeSalidaDeclarada("tag", "v(.+)", true)
	require.NoError(t, err)
	asercion, err := dominio.NuevaAsercionDeclarada("OK")
	require.NoError(t, err)

	c, err := dominio.NuevoComandoDeclarado(
		"build", "docker build .", "docker", []string{"Dockerfile"},
		[]dominio.VariableDeSalidaDeclarada{salida}, []dominio.AsercionDeclarada{asercion},
	)
	require.NoError(t, err)
	require.Equal(t, "build", c.Nombre())
	require.Equal(t, "docker build .", c.Linea())
	require.Equal(t, "docker", c.Directorio())
	require.Equal(t, []string{"Dockerfile"}, c.Plantillas())
	require.Len(t, c.Salidas(), 1)
	require.Len(t, c.Aserciones(), 1)
}

func TestUnaVariableDeSalidaDeclaradaNoPuedeTenerElNombreVacio(t *testing.T) {
	_, err := dominio.NuevaVariableDeSalidaDeclarada("", "v(.+)", false)
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnaVariableDeSalidaDeclaradaNecesitaSuExpresion(t *testing.T) {
	_, err := dominio.NuevaVariableDeSalidaDeclarada("tag", "", false)
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnaAsercionDeclaradaNoPuedeTenerLaExpresionVacia(t *testing.T) {
	_, err := dominio.NuevaAsercionDeclarada("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnFicheroDeclaradoNoPuedeTenerLaRutaVacia(t *testing.T) {
	_, err := dominio.NuevoFicheroDeclarado("", "contenido", false, "", false)
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnFicheroDeclaradoGuardaLoQueDeclara(t *testing.T) {
	f, err := dominio.NuevoFicheroDeclarado("k8s/deployment.yaml", "contenido", true, "", true)
	require.NoError(t, err)
	require.Equal(t, "k8s/deployment.yaml", f.Ruta())
	require.Equal(t, "contenido", f.Contenido())
	require.True(t, f.Ejecutable())
	require.Empty(t, f.Enlace())
	require.True(t, f.Plantilla())
}
