package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

func TestHashesDeAhoraIgualesALosDeLaUltimaVezNoSonCambio(t *testing.T) {
	ahora := map[string]dominio.HashDeVariable{"n": dominio.CalcularHashDeVariable("v")}
	ultimaVez := map[string]dominio.HashDeVariable{"n": dominio.CalcularHashDeVariable("v")}
	require.False(t, dominio.Cambiaron(ahora, ultimaVez))
}

func TestUnHashDistintoEsCambio(t *testing.T) {
	ahora := map[string]dominio.HashDeVariable{"n": dominio.CalcularHashDeVariable("nuevo")}
	ultimaVez := map[string]dominio.HashDeVariable{"n": dominio.CalcularHashDeVariable("viejo")}
	require.True(t, dominio.Cambiaron(ahora, ultimaVez))
}

func TestDistintaCantidadDeNombresEsCambio(t *testing.T) {
	ahora := map[string]dominio.HashDeVariable{
		"n": dominio.CalcularHashDeVariable("v"), "m": dominio.CalcularHashDeVariable("v"),
	}
	ultimaVez := map[string]dominio.HashDeVariable{"n": dominio.CalcularHashDeVariable("v")}
	require.True(t, dominio.Cambiaron(ahora, ultimaVez))
}

func TestUnNombreDistintoConLaMismaCantidadEsCambio(t *testing.T) {
	ahora := map[string]dominio.HashDeVariable{"n": dominio.CalcularHashDeVariable("v")}
	ultimaVez := map[string]dominio.HashDeVariable{"m": dominio.CalcularHashDeVariable("v")}
	require.True(t, dominio.Cambiaron(ahora, ultimaVez))
}

func TestSinNombresNoHayCambio(t *testing.T) {
	require.False(t, dominio.Cambiaron(map[string]dominio.HashDeVariable{}, map[string]dominio.HashDeVariable{}))
}
