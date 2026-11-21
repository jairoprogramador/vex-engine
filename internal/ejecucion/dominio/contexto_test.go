package dominio_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func contexto(t *testing.T, valor string) dominio.ContextoDeEjecucion {
	t.Helper()
	c, err := dominio.NuevoContexto(valor)
	require.NoError(t, err)
	return c
}

func TestSinContextoLaHuellaDeVariablesEsLaDeResolucion(t *testing.T) {
	huella := dominio.NuevoHashDeVariables("entradas-v2:abc")

	require.Equal(t, huella, contexto(t, "").ConLasVariables(huella), "byte a byte: quien no manda contexto conserva su historial")
	require.Equal(t, huella, dominio.ContextoDeEjecucion{}.ConLasVariables(huella), "el valor cero es sin contexto")
}

func TestElContextoSeMezclaEnLaHuellaDeVariables(t *testing.T) {
	huella := dominio.NuevoHashDeVariables("entradas-v2:abc")
	a := contexto(t, "azure/v1:aaaa").ConLasVariables(huella)

	require.Equal(t, a, contexto(t, "azure/v1:aaaa").ConLasVariables(huella), "el mismo contexto da la misma huella")
	require.NotEqual(t, huella, a, "con contexto no es la de Resolución")
	require.NotEqual(t, a, contexto(t, "azure/v1:bbbb").ConLasVariables(huella), "otro contexto, otra huella")
	require.NotEqual(t, a, contexto(t, "azure/v1:aaaa").ConLasVariables(dominio.NuevoHashDeVariables("entradas-v2:abd")),
		"y las variables siguen contando")
	require.NotContains(t, a.String(), "azure/v1:aaaa", "la cadena del contexto no queda en lo que se guarda")
}

func TestUnContextoAdmiteLoQueMandaUnCliente(t *testing.T) {
	for _, valor := range []string{
		"", "x", "azure/v1:3f9a1c0d2b7e4a65", "a.b_c:d/e@f+g=h-i", "ABCxyz0189", strings.Repeat("a", 128),
	} {
		_, err := dominio.NuevoContexto(valor)
		require.NoError(t, err, "%q", valor)
	}
}

func TestUnContextoQueNoValeEsInvalidoYNoCuentaSuValor(t *testing.T) {
	casos := map[string]string{
		"129 bytes":                 strings.Repeat("a", 129),
		"un espacio":                "con espacio",
		"un salto de línea":         "azure\nv1",
		"un punto y coma":           "azure;v1",
		"un NUL":                    "azure\x00v1",
		"una comilla":               `azure"v1`,
		"una letra que no es ASCII": "contraseña",
		"más de 128 bytes aunque sean menos caracteres": strings.Repeat("ñ", 65),
	}
	for nombre, valor := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := dominio.NuevoContexto(valor)

			require.ErrorIs(t, err, dominio.ErrInvalido)
			require.NotContains(t, err.Error(), valor, "el mensaje no incluye el valor")
			var parametro *dominio.ParametroInvalidoError
			require.True(t, errors.As(err, &parametro))
			campo, enElError := parametro.ParametroInvalido()
			require.Equal(t, "Contexto", campo)
			require.Empty(t, enElError, "ni lo lleva como dato")
		})
	}
}
