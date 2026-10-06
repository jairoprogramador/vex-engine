package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

func ambienteProd(t *testing.T) dominio.Ambito {
	t.Helper()
	a, err := dominio.AmbitoDeAmbiente("prod")
	require.NoError(t, err)
	return a
}

func TestUnAmbitoDeAmbienteNoPuedeTenerElNombreVacio(t *testing.T) {
	_, err := dominio.AmbitoDeAmbiente("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnPasoDeUnAmbienteVeLoCompartidoYLoSuyo(t *testing.T) {
	desde := ambienteProd(t)
	require.True(t, ambienteProd(t).Ve(desde))
	require.True(t, dominio.AmbitoCompartido().Ve(desde))
}

func TestUnPasoDeUnAmbienteNoVeElDeOtroAmbiente(t *testing.T) {
	stag, err := dominio.AmbitoDeAmbiente("stag")
	require.NoError(t, err)
	require.False(t, stag.Ve(ambienteProd(t)))
}

func TestElCompartidoNoVeLoDeNingunAmbiente(t *testing.T) {
	require.False(t, ambienteProd(t).Ve(dominio.AmbitoCompartido()))
}

func TestElCompartidoSeVeDesdeSiMismo(t *testing.T) {
	require.True(t, dominio.AmbitoCompartido().Ve(dominio.AmbitoCompartido()))
}

func TestUnaVariableEfectivaNoPuedeTenerElNombreVacio(t *testing.T) {
	_, err := dominio.NuevaVariableEfectiva("", "valor", dominio.OrigenDeclarada, ambienteProd(t))
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnaVariableEfectivaNecesitaUnOrigenValido(t *testing.T) {
	_, err := dominio.NuevaVariableEfectiva("n", "v", dominio.Origen(0), ambienteProd(t))
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnaVariableEfectivaNecesitaUnAmbito(t *testing.T) {
	_, err := dominio.NuevaVariableEfectiva("n", "v", dominio.OrigenDeclarada, dominio.Ambito{})
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestNingunMetodoDeVariableEfectivaMuestraElValorEnString(t *testing.T) {
	v, err := dominio.NuevaVariableEfectiva("secreta", "un-valor-muy-secreto", dominio.OrigenDeclarada, ambienteProd(t))
	require.NoError(t, err)
	require.NotContains(t, v.String(), "un-valor-muy-secreto")
	require.Equal(t, "un-valor-muy-secreto", v.Valor())
}

func TestUnHashDeVariableNoPuedeEstarVacio(t *testing.T) {
	_, err := dominio.NuevoHashDeVariable("")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestElMismoValorDaElMismoHashSiempre(t *testing.T) {
	require.Equal(t, dominio.CalcularHashDeVariable("hola"), dominio.CalcularHashDeVariable("hola"))
}

func TestDosValoresDistintosDanHashesDistintos(t *testing.T) {
	require.NotEqual(t, dominio.CalcularHashDeVariable("hola"), dominio.CalcularHashDeVariable("adios"))
}

func TestElHashDeVariableLlevaSuPrefijoDeVersion(t *testing.T) {
	require.Contains(t, dominio.CalcularHashDeVariable("hola").String(), "variable-v1:")
}
