package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestUnHashDeCodigoNoPuedeEstarVacio(t *testing.T) {
	_, err := dominio.NuevoHashDeCodigo("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnHashDeInstruccionesNoPuedeEstarVacio(t *testing.T) {
	_, err := dominio.NuevoHashDeInstrucciones("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestDosHashesDeCodigoIgualesSonElMismoValor(t *testing.T) {
	a, err := dominio.NuevoHashDeCodigo("contenido-v1:abc")
	require.NoError(t, err)
	b, err := dominio.NuevoHashDeCodigo("contenido-v1:abc")
	require.NoError(t, err)
	require.Equal(t, a, b)
}

func TestLosRecursosDeUnPasoGuardanLosTresDatos(t *testing.T) {
	codigo, err := dominio.NuevoHashDeCodigo("h1")
	require.NoError(t, err)
	instrucciones, err := dominio.NuevoHashDeInstrucciones("h2")
	require.NoError(t, err)

	r := dominio.NuevosRecursosDeUnPaso(codigo, instrucciones, true)
	require.Equal(t, codigo, r.HashDelCodigo())
	require.Equal(t, instrucciones, r.HashDeInstrucciones())
	require.True(t, r.CambiaronVariables())
}
