package aplicacion_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
)

func registrado(t *testing.T, id, ambiente, despliegue string, numero int, nombre string) dominio.LanzamientoRegistrado {
	t.Helper()
	return dominio.LanzamientoRegistrado{
		Id: id, Ambiente: mustAmbiente(t, ambiente), Despliegue: idDespliegue(t, despliegue),
		Version: version(t, numero), Nombre: dominio.Nombre(nombre),
		Instante: time.Date(2026, 10, 6, 10, numero, 0, 0, time.UTC),
	}
}

func TestLanzamientosDeUnAmbiente_ListaSoloLosDeEseAmbienteConSuId(t *testing.T) {
	h := nuevoHistorialFalso()
	h.registrados = []dominio.LanzamientoRegistrado{
		registrado(t, "lz-1", "staging", "d1", 1, "uno"),
		registrado(t, "lz-2", "prod", "d2", 2, "dos"),
		registrado(t, "lz-3", "staging", "d3", 3, "tres"),
	}

	lanzamientos, err := nuevoServicio(h).LanzamientosDeUnAmbiente(context.Background(), "staging")

	require.NoError(t, err)
	require.Equal(t, []publicado.Lanzamiento{
		{Id: "lz-1", Ambiente: "staging", Despliegue: "d1", Version: 1, Nombre: "uno", Instante: h.registrados[0].Instante},
		{Id: "lz-3", Ambiente: "staging", Despliegue: "d3", Version: 3, Nombre: "tres", Instante: h.registrados[2].Instante},
	}, lanzamientos)
}

func TestLanzamientosDeUnAmbiente_SinLanzamientosEsUnaListaVacia(t *testing.T) {
	lanzamientos, err := nuevoServicio(nuevoHistorialFalso()).LanzamientosDeUnAmbiente(context.Background(), "prod")

	require.NoError(t, err)
	require.NotNil(t, lanzamientos)
	require.Empty(t, lanzamientos)
}

func TestLanzamientosDeUnAmbiente_AmbienteVacioEsInvalidoYDiceElCampo(t *testing.T) {
	_, err := nuevoServicio(nuevoHistorialFalso()).LanzamientosDeUnAmbiente(context.Background(), "")

	require.ErrorIs(t, err, publicado.ErrInvalido)
	var parametro interface{ ParametroInvalido() (string, string) }
	require.ErrorAs(t, err, &parametro)
	campo, valor := parametro.ParametroInvalido()
	require.Equal(t, "Ambiente", campo)
	require.Empty(t, valor)
}

func TestLanzamientosDeUnAmbiente_UnaFallaDelHistorialSePropagaSinSerInvalido(t *testing.T) {
	h := nuevoHistorialFalso()
	h.fallarLanzamientosDeUnAmbiente = true

	_, err := nuevoServicio(h).LanzamientosDeUnAmbiente(context.Background(), "prod")

	require.Error(t, err)
	require.NotErrorIs(t, err, publicado.ErrInvalido)
}
