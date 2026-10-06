package dominio_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestUnaReglaNoPuedeTenerLaEdadMaximaNegativa(t *testing.T) {
	_, err := dominio.NuevaRegla(true, false, false, -time.Minute)
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnaReglaVaciaEsValida(t *testing.T) {
	r, err := dominio.NuevaRegla(false, false, false, 0)
	require.NoError(t, err)
	require.False(t, r.MiraCodigo())
	require.False(t, r.MiraInstrucciones())
	require.False(t, r.MiraVariables())
	require.Zero(t, r.EdadMaxima())
}

func TestUnaReglaGuardaLoQueMira(t *testing.T) {
	r, err := dominio.NuevaRegla(true, true, false, 30*time.Minute)
	require.NoError(t, err)
	require.True(t, r.MiraCodigo())
	require.True(t, r.MiraInstrucciones())
	require.False(t, r.MiraVariables())
	require.Equal(t, 30*time.Minute, r.EdadMaxima())
}
