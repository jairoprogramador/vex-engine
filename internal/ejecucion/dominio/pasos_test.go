package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestUnPasoDelPipelineNoPuedeTenerElNombreVacio(t *testing.T) {
	_, err := dominio.NuevoPasoDelPipeline("", false)
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnPasoDelPipelineGuardaSuNombreYSuAmbito(t *testing.T) {
	p, err := dominio.NuevoPasoDelPipeline("01-pruebas", true)
	require.NoError(t, err)
	require.Equal(t, "01-pruebas", p.Nombre())
	require.True(t, p.Compartido())
}
