package aplicacion_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// TestPeticionSinIntento_InterpolaSinCalcularHashNiEscribirNada usa un Historial que hace fallar la prueba si
// se le llama: el camino de simulación no debe tocarlo nunca (DEC-04.10).
func TestPeticionSinIntento_InterpolaSinCalcularHashNiEscribirNada(t *testing.T) {
	h := nuevoHistorialFalso(t)
	h.prohibido = true
	s := nuevoServicio(t, h, &definicionFalsa{})
	ctx := context.Background()
	prod := ambitoProdPublicado()

	err := s.ParaSimulacion().Declarar(ctx, "simulacion-1", prod, []publicado.VariableDeclarada{
		{Nombre: "host", Ambito: prod, Valor: "vexja.com"},
	})
	require.NoError(t, err)

	texto, err := s.ParaSimulacion().Interpolar(ctx, "simulacion-1", prod, "https://${var.host}/")
	require.NoError(t, err)
	require.Equal(t, "https://vexja.com/", texto)

	require.NoError(t, s.ParaSimulacion().RegistrarProducido(ctx, "simulacion-1", "puerto", "443", prod))

	texto, err = s.ParaSimulacion().Interpolar(ctx, "simulacion-1", prod, "${var.host}:${var.puerto}")
	require.NoError(t, err)
	require.Equal(t, "vexja.com:443", texto)
}

func TestPeticionSinIntento_CerrarLiberaLaMemoria(t *testing.T) {
	s := nuevoServicio(t, nil, nil)
	ctx := context.Background()
	prod := ambitoProdPublicado()

	require.NoError(t, s.ParaSimulacion().Declarar(ctx, "simulacion-1", prod, []publicado.VariableDeclarada{
		{Nombre: "host", Ambito: prod, Valor: "vexja.com"},
	}))
	require.NoError(t, s.ParaSimulacion().Cerrar(ctx, "simulacion-1"))

	// tras cerrar, la memoria se liberó: la misma simulación vuelve a empezar vacía.
	_, err := s.ParaSimulacion().Interpolar(ctx, "simulacion-1", prod, "${var.host}")
	require.Error(t, err)
}

func TestElValorNuncaCruzaHaciaLoPublicado(t *testing.T) {
	def := &definicionFalsa{declaradas: []dominio.VariableDeclarada{
		{Nombre: "secreta", Ambito: mustAmbito(t, "prod"), Valor: "un-valor-muy-secreto"},
	}}
	s := nuevoServicio(t, nil, def)
	ctx := context.Background()

	variables, err := s.ParaEjecucion().VariablesDeUnPaso(
		ctx, "intento-1", "paso", ambitoProdPublicado(), "fuente", "commit", nil)
	require.NoError(t, err)

	datos, err := json.Marshal(variables)
	require.NoError(t, err)
	require.NotContains(t, string(datos), "un-valor-muy-secreto")
}
