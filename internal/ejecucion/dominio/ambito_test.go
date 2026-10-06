package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestUnAmbitoDeAmbienteNoPuedeTenerElNombreVacio(t *testing.T) {
	_, err := dominio.AmbitoDeAmbiente("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestElAmbitoDeUnPasoCompartidoEsElCompartido(t *testing.T) {
	paso, err := dominio.NuevoPasoDelPipeline("01-pruebas", true)
	require.NoError(t, err)
	a, err := dominio.AmbitoDelPaso(paso, "prod")
	require.NoError(t, err)
	require.True(t, a.EsCompartido())
}

func TestElAmbitoDeUnPasoNoCompartidoEsElDelAmbiente(t *testing.T) {
	paso, err := dominio.NuevoPasoDelPipeline("02-despliegue", false)
	require.NoError(t, err)
	a, err := dominio.AmbitoDelPaso(paso, "prod")
	require.NoError(t, err)
	require.False(t, a.EsCompartido())
	require.Equal(t, "prod", a.Ambiente())
}
