package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

func TestNuevaAmbiente_RechazaElVacio(t *testing.T) {
	_, err := dominio.NuevaAmbiente("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestNuevoIdDespliegue_RechazaElVacio(t *testing.T) {
	_, err := dominio.NuevoIdDespliegue("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestNuevoHashDelCodigo_RechazaElVacio(t *testing.T) {
	_, err := dominio.NuevoHashDelCodigo("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestNuevaVersion_RechazaMenorQueUno(t *testing.T) {
	_, err := dominio.NuevaVersion(0)
	require.ErrorIs(t, err, dominio.ErrInvalido)
	_, err = dominio.NuevaVersion(-1)
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestNuevaVersion_AceptaUnoYMas(t *testing.T) {
	v, err := dominio.NuevaVersion(1)
	require.NoError(t, err)
	require.Equal(t, 1, v.Numero())
	require.Equal(t, "1", v.String())
}

func TestNombre_Vacia(t *testing.T) {
	require.True(t, dominio.Nombre("").Vacia())
	require.False(t, dominio.Nombre("v1.2").Vacia())
}
