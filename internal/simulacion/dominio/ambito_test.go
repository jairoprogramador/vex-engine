package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

func TestAmbitoDeAmbienteVacioEsInvalido(t *testing.T) {
	_, err := dominio.AmbitoDeAmbiente("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestAmbitoDelPaso(t *testing.T) {
	compartido, err := dominio.AmbitoDelPaso(true, "sand")
	require.NoError(t, err)
	require.True(t, compartido.EsCompartido())

	propio, err := dominio.AmbitoDelPaso(false, "sand")
	require.NoError(t, err)
	require.False(t, propio.EsCompartido())
	require.Equal(t, "sand", propio.Ambiente())

	_, err = dominio.AmbitoDelPaso(false, "")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}
