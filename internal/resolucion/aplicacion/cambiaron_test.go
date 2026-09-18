package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

func TestCambiaronLasVariables_SinUltimaVezSiempreCambio(t *testing.T) {
	s := nuevoServicio(t, nil, nil)
	ctx := context.Background()

	cambiaron, err := s.ParaEjecucion().CambiaronLasVariables(ctx, "intento-1", "paso", ambitoProdPublicado())
	require.NoError(t, err)
	require.True(t, cambiaron)
}

func TestCambiaronLasVariables_MismoHashNoEsCambio(t *testing.T) {
	h := nuevoHistorialFalso(t)
	def := &definicionFalsa{estandar: []dominio.VariableEstandar{{Nombre: "n", DelPaso: true}}}
	s := nuevoServicio(t, h, def)
	ctx := context.Background()

	_, err := s.ParaEjecucion().VariablesDeUnPaso(
		ctx, "intento-1", "paso", ambitoProdPublicado(), "fuente", "commit", map[string]string{"n": "v"})
	require.NoError(t, err)

	h.ultimaVez[h.clave("paso", mustAmbito(t, "prod"))] = map[string]dominio.HashDeVariable{
		"n": dominio.CalcularHashDeVariable("v"),
	}

	cambiaron, err := s.ParaEjecucion().CambiaronLasVariables(ctx, "intento-1", "paso", ambitoProdPublicado())
	require.NoError(t, err)
	require.False(t, cambiaron)
}

func TestCambiaronLasVariables_UnHashDistintoEsCambio(t *testing.T) {
	h := nuevoHistorialFalso(t)
	def := &definicionFalsa{estandar: []dominio.VariableEstandar{{Nombre: "n", DelPaso: true}}}
	s := nuevoServicio(t, h, def)
	ctx := context.Background()

	_, err := s.ParaEjecucion().VariablesDeUnPaso(
		ctx, "intento-1", "paso", ambitoProdPublicado(), "fuente", "commit", map[string]string{"n": "nuevo"})
	require.NoError(t, err)

	h.ultimaVez[h.clave("paso", mustAmbito(t, "prod"))] = map[string]dominio.HashDeVariable{
		"n": dominio.CalcularHashDeVariable("viejo"),
	}

	cambiaron, err := s.ParaEjecucion().CambiaronLasVariables(ctx, "intento-1", "paso", ambitoProdPublicado())
	require.NoError(t, err)
	require.True(t, cambiaron)
}
