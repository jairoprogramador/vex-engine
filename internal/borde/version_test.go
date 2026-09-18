package borde

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComprobarVersionAceptaUnaVersionSoportada(t *testing.T) {
	require.NoError(t, comprobarVersion("1"))
}

func TestComprobarVersionRechazaUnaVersionNoSoportada(t *testing.T) {
	err := comprobarVersion("99")
	require.ErrorIs(t, err, ErrVersionNoSoportada)
}
