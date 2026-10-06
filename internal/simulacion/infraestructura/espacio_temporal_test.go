package infraestructura_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/simulacion/infraestructura"
)

func TestEspacioTemporalDaIdsDistintos(t *testing.T) {
	e := infraestructura.EspacioTemporal{}

	primero, err := e.Nuevo()
	require.NoError(t, err)
	require.NotEmpty(t, primero)

	segundo, err := e.Nuevo()
	require.NoError(t, err)
	require.NotEqual(t, primero, segundo)
}
