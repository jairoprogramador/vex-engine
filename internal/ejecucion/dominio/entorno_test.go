package dominio_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestNuevoEntorno_UnosNombresValidosConSusValores(t *testing.T) {
	entorno, err := dominio.NuevoEntorno(map[string]string{
		"REGISTRY_URL": "registry.local", "_privada": "x", "a1": "", "ConValorConIgual": "a=b=c", "ConSaltos": "una\nlinea",
	})

	require.NoError(t, err)
	require.False(t, entorno.Vacio())
	require.Equal(t, []string{
		"ConSaltos=una\nlinea", "ConValorConIgual=a=b=c", "REGISTRY_URL=registry.local", "_privada=x", "a1=",
	}, entorno.Lista(), "NOMBRE=valor, ordenadas: siempre el mismo entorno para el mismo mapa")
}

func TestNuevoEntorno_SinVariablesEsVacioYSuListaTambien(t *testing.T) {
	for nombre, variables := range map[string]map[string]string{"nil": nil, "sin elementos": {}} {
		t.Run(nombre, func(t *testing.T) {
			entorno, err := dominio.NuevoEntorno(variables)

			require.NoError(t, err)
			require.True(t, entorno.Vacio())
			require.Empty(t, entorno.Lista())
		})
	}
	var cero dominio.Entorno
	require.True(t, cero.Vacio(), "el valor cero es un entorno sin variables")
	require.Empty(t, cero.Lista())
}

func TestNuevoEntorno_UnNombreQueNoEsDeVariableDeEntornoEsInvalido(t *testing.T) {
	for _, nombre := range []string{"", "1A", "A-B", "A=B", "con espacio", "ñandú", "A.B", "A\x00B", " A"} {
		t.Run(fmt.Sprintf("%q", nombre), func(t *testing.T) {
			_, err := dominio.NuevoEntorno(map[string]string{nombre: "v"})

			require.ErrorIs(t, err, dominio.ErrInvalido)
		})
	}
}

func TestNuevoEntorno_UnValorConNULEsInvalido(t *testing.T) {
	_, err := dominio.NuevoEntorno(map[string]string{"A": "antes\x00despues"})

	require.ErrorIs(t, err, dominio.ErrInvalido, "el sistema operativo no puede pasar un valor así")
	require.ErrorContains(t, err, `"A"`, "dice qué variable")
}

func TestNuevoEntorno_UnErrorDiceElNombreNuncaElValor(t *testing.T) {
	_, err := dominio.NuevoEntorno(map[string]string{"NOMBRE-MALO": "valor-sensible-123"})

	require.ErrorContains(t, err, "NOMBRE-MALO")
	require.NotContains(t, err.Error(), "valor-sensible-123")
}

func TestEntorno_NoCambiaSiSeCambiaElMapaConElQueSeCreo(t *testing.T) {
	variables := map[string]string{"A": "1"}
	entorno, err := dominio.NuevoEntorno(variables)
	require.NoError(t, err)

	variables["A"] = "2"
	variables["B"] = "3"

	require.Equal(t, []string{"A=1"}, entorno.Lista())
}

func TestEntorno_AlImprimirloSoloDiceLosNombres(t *testing.T) {
	entorno, err := dominio.NuevoEntorno(map[string]string{"TOKEN": "valor-sensible-123", "URL": "otro-valor"})
	require.NoError(t, err)

	for _, texto := range []string{fmt.Sprint(entorno), fmt.Sprintf("%v", entorno), fmt.Sprintf("%+v", entorno), fmt.Sprintf("%#v", entorno), fmt.Sprintf("%s", entorno)} {
		require.Contains(t, texto, "TOKEN")
		require.Contains(t, texto, "URL")
		require.NotContains(t, texto, "valor-sensible-123", "un entorno que acaba en un log o en un error no cuenta sus valores")
		require.NotContains(t, texto, "otro-valor")
	}
}
