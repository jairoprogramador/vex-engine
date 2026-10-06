package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

func declarada(t *testing.T, nombre, valor string, ambito dominio.Ambito) dominio.VariableEfectiva {
	t.Helper()
	v, err := dominio.NuevaVariableEfectiva(nombre, valor, dominio.OrigenDeclarada, ambito)
	require.NoError(t, err)
	return v
}

func producida(t *testing.T, nombre, valor string, ambito dominio.Ambito) dominio.VariableEfectiva {
	t.Helper()
	v, err := dominio.NuevaVariableEfectiva(nombre, valor, dominio.OrigenProducida, ambito)
	require.NoError(t, err)
	return v
}

func TestDeclararRechazaUnaVariableQueNoEsDeOrigenDeclarada(t *testing.T) {
	inv := dominio.NuevaInvocacion()
	err := inv.Declarar(producida(t, "n", "v", ambienteProd(t)))
	require.ErrorIs(t, err, dominio.ErrRechazado)
}

func TestProducirRechazaUnaVariableQueNoEsDeOrigenProducida(t *testing.T) {
	inv := dominio.NuevaInvocacion()
	err := inv.Producir(declarada(t, "n", "v", ambienteProd(t)))
	require.ErrorIs(t, err, dominio.ErrRechazado)
}

func TestUnaDeclaradaNoSobreescribeLoQueYaHay(t *testing.T) {
	inv := dominio.NuevaInvocacion()
	require.NoError(t, inv.Declarar(declarada(t, "n", "primera", ambienteProd(t))))
	require.NoError(t, inv.Declarar(declarada(t, "n", "segunda", ambienteProd(t))))

	visibles := inv.Visibles(ambienteProd(t))
	require.Len(t, visibles, 1)
	require.Equal(t, "primera", visibles[0].Valor())
}

func TestUnaVariableProducidaGanaAUnaDeclarada(t *testing.T) {
	inv := dominio.NuevaInvocacion()
	require.NoError(t, inv.Declarar(declarada(t, "n", "declarada", ambienteProd(t))))
	require.NoError(t, inv.Producir(producida(t, "n", "producida", ambienteProd(t))))

	visibles := inv.Visibles(ambienteProd(t))
	require.Len(t, visibles, 1)
	require.Equal(t, "producida", visibles[0].Valor())
	require.Equal(t, dominio.OrigenProducida, visibles[0].Origen())
}

func TestEntreProducidasGanaLaDelPasoMasReciente(t *testing.T) {
	inv := dominio.NuevaInvocacion()
	require.NoError(t, inv.Producir(producida(t, "n", "primer-paso", ambienteProd(t))))
	require.NoError(t, inv.Producir(producida(t, "n", "segundo-paso", ambienteProd(t))))

	visibles := inv.Visibles(ambienteProd(t))
	require.Len(t, visibles, 1)
	require.Equal(t, "segundo-paso", visibles[0].Valor())
}

func TestVisiblesRespetaElAmbito(t *testing.T) {
	inv := dominio.NuevaInvocacion()
	stag, err := dominio.AmbitoDeAmbiente("stag")
	require.NoError(t, err)

	require.NoError(t, inv.Declarar(declarada(t, "de-prod", "v", ambienteProd(t))))
	require.NoError(t, inv.Declarar(declarada(t, "de-stag", "v", stag)))
	require.NoError(t, inv.Declarar(declarada(t, "compartida", "v", dominio.AmbitoCompartido())))

	visibles := inv.Visibles(ambienteProd(t))
	var nombres []string
	for _, v := range visibles {
		nombres = append(nombres, v.Nombre())
	}
	require.ElementsMatch(t, []string{"de-prod", "compartida"}, nombres)
}

func TestVisiblesConservaElOrdenDePrimeraAparicion(t *testing.T) {
	inv := dominio.NuevaInvocacion()
	require.NoError(t, inv.Declarar(declarada(t, "b", "v", ambienteProd(t))))
	require.NoError(t, inv.Declarar(declarada(t, "a", "v", ambienteProd(t))))
	require.NoError(t, inv.Producir(producida(t, "b", "v2", ambienteProd(t))))

	visibles := inv.Visibles(ambienteProd(t))
	require.Len(t, visibles, 2)
	require.Equal(t, "b", visibles[0].Nombre())
	require.Equal(t, "a", visibles[1].Nombre())
}
