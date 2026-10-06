package aplicacion_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

func TestRegistrarProducido_GanaALaDeclaradaDesdeElSiguienteComando(t *testing.T) {
	def := &definicionFalsa{declaradas: []dominio.VariableDeclarada{
		{Nombre: "n", Ambito: mustAmbito(t, "prod"), Valor: "declarado"},
	}}
	s := nuevoServicio(t, nil, def)
	ctx := context.Background()

	_, err := s.ParaEjecucion().VariablesDeUnPaso(
		ctx, "intento-1", "paso", ambitoProdPublicado(), "fuente", "commit", nil)
	require.NoError(t, err)

	require.NoError(t, s.ParaEjecucion().RegistrarProducido(
		ctx, "intento-1", "paso", "n", "producido", ambitoProdPublicado()))

	texto, err := s.ParaEjecucion().Interpolar(ctx, "intento-1", "paso", ambitoProdPublicado(), "${var.n}")
	require.NoError(t, err)
	require.Equal(t, "producido", texto)
}

func TestRegistrarProducido_EscribeElHashAntesQueElValor(t *testing.T) {
	h := nuevoHistorialFalso(t)
	s := nuevoServicio(t, h, nil)
	ctx := context.Background()

	require.NoError(t, s.ParaEjecucion().RegistrarProducido(ctx, "intento-1", "paso", "n", "v", ambitoProdPublicado()))
	require.Equal(t, []string{"hash:n", "valor:n"}, h.orden)
}

func TestRegistrarProducido_SiGuardarValorFallaNoQuedaProducidaEnMemoria(t *testing.T) {
	h := nuevoHistorialFalso(t)
	h.fallarGuardarValor = true
	s := nuevoServicio(t, h, nil)
	ctx := context.Background()

	err := s.ParaEjecucion().RegistrarProducido(ctx, "intento-1", "paso", "n", "v", ambitoProdPublicado())
	require.Error(t, err)
	require.Len(t, h.registros, 1) // el hash sí quedó escrito
	require.Empty(t, h.valoresGuardados)

	_, err = s.ParaEjecucion().Interpolar(ctx, "intento-1", "paso", ambitoProdPublicado(), "${var.n}")
	var noEncontrada *publicado.VariableNoEncontradaError
	require.ErrorAs(t, err, &noEncontrada)
	require.Equal(t, "n", noEncontrada.Nombre)
}

func TestRegistrarProducido_SiRegistrarVariableFallaNoIntentaGuardarValor(t *testing.T) {
	h := nuevoHistorialFalso(t)
	h.fallarRegistrarVariable = true
	s := nuevoServicio(t, h, nil)
	ctx := context.Background()

	err := s.ParaEjecucion().RegistrarProducido(ctx, "intento-1", "paso", "n", "v", ambitoProdPublicado())
	require.Error(t, err)
	require.Empty(t, h.valoresGuardados)
}
