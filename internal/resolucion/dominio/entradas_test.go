package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

func variable(t *testing.T, nombre, valor string) dominio.VariableEfectiva {
	t.Helper()
	v, err := dominio.NuevaVariableEfectiva(nombre, valor, dominio.OrigenDeclarada, dominio.AmbitoCompartido())
	require.NoError(t, err)
	return v
}

func TestHashDeLasEntradas_LasMismasVariablesDanElMismoHashSinImportarElOrden(t *testing.T) {
	a := dominio.HashDeLasEntradas([]dominio.VariableEfectiva{variable(t, "a", "1"), variable(t, "b", "2")})
	b := dominio.HashDeLasEntradas([]dominio.VariableEfectiva{variable(t, "b", "2"), variable(t, "a", "1")})
	require.Equal(t, a, b)
}

func TestHashDeLasEntradas_UnValorDistintoCambiaElHash(t *testing.T) {
	antes := dominio.HashDeLasEntradas([]dominio.VariableEfectiva{variable(t, "n", "viejo")})
	ahora := dominio.HashDeLasEntradas([]dominio.VariableEfectiva{variable(t, "n", "nuevo")})
	require.NotEqual(t, antes, ahora)
}

func TestHashDeLasEntradas_UnNombreAñadidoOQuitadoCambiaElHash(t *testing.T) {
	una := dominio.HashDeLasEntradas([]dominio.VariableEfectiva{variable(t, "a", "1")})
	dos := dominio.HashDeLasEntradas([]dominio.VariableEfectiva{variable(t, "a", "1"), variable(t, "b", "2")})
	otroNombre := dominio.HashDeLasEntradas([]dominio.VariableEfectiva{variable(t, "b", "1")})
	require.NotEqual(t, una, dos)
	require.NotEqual(t, una, otroNombre)
}

func TestHashDeLasEntradas_SinEntradasTieneHashYNoEsVacio(t *testing.T) {
	require.NotEmpty(t, dominio.HashDeLasEntradas(nil).String())
}
