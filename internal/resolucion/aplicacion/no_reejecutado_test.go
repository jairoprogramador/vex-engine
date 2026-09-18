package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

func TestNoReejecutado_AportaLasVariablesDeLaUltimaVezComoProducidas(t *testing.T) {
	h := nuevoHistorialFalso(t)
	h.valoresUltimaVez[h.clave("paso", mustAmbito(t, "prod"))] = map[string]dominio.ValorDeLaUltimaVez{
		"n": {Valor: "v", Ambito: mustAmbito(t, "prod")},
	}
	s := nuevoServicio(t, h, nil)
	ctx := context.Background()

	require.NoError(t, s.ParaEjecucion().NoReejecutado(ctx, "intento-1", "paso", ambitoProdPublicado()))

	texto, err := s.ParaEjecucion().Interpolar(ctx, "intento-1", "paso", ambitoProdPublicado(), "${var.n}")
	require.NoError(t, err)
	require.Equal(t, "v", texto)
}

func TestNoReejecutado_ConservaElAmbitoConElQueSeProdujo(t *testing.T) {
	h := nuevoHistorialFalso(t)
	h.valoresUltimaVez[h.clave("paso-compartido", dominio.AmbitoCompartido())] = map[string]dominio.ValorDeLaUltimaVez{
		"compartida": {Valor: "v", Ambito: dominio.AmbitoCompartido()},
	}
	s := nuevoServicio(t, h, nil)
	ctx := context.Background()

	require.NoError(t, s.ParaEjecucion().NoReejecutado(ctx, "intento-1", "paso-compartido", ambitoCompartidoPublicado()))

	// visible también desde otro ambiente, porque se produjo en el ámbito compartido.
	texto, err := s.ParaEjecucion().Interpolar(ctx, "intento-1", "paso", ambitoProdPublicado(), "${var.compartida}")
	require.NoError(t, err)
	require.Equal(t, "v", texto)
}

func TestNoReejecutado_NoEscribeNadaEnElHistorial(t *testing.T) {
	h := nuevoHistorialFalso(t)
	h.valoresUltimaVez[h.clave("paso", mustAmbito(t, "prod"))] = map[string]dominio.ValorDeLaUltimaVez{
		"n": {Valor: "v", Ambito: mustAmbito(t, "prod")},
	}
	s := nuevoServicio(t, h, nil)
	ctx := context.Background()

	require.NoError(t, s.ParaEjecucion().NoReejecutado(ctx, "intento-1", "paso", ambitoProdPublicado()))
	require.Empty(t, h.registros)
	require.Empty(t, h.valoresGuardados)
}

func TestNoReejecutado_SinUltimaVezFalla(t *testing.T) {
	s := nuevoServicio(t, nil, nil)
	ctx := context.Background()

	err := s.ParaEjecucion().NoReejecutado(ctx, "intento-1", "paso", ambitoProdPublicado())
	require.Error(t, err)
}
