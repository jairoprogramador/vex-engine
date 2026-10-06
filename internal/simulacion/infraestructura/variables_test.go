package infraestructura_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/infraestructura"
)

// El adaptador de Variables se prueba contra un doble de resolucion/publicado.ParaSimulacion (DEC-11.3).
type variablesFalsas struct {
	simulacionRecibida  string
	ambitoRecibido      resolucionpublicado.Ambito
	declaradasRecibidas []resolucionpublicado.VariableDeclarada
	interpolado         string
	cerrada             bool
	err                 error
}

func (v *variablesFalsas) Declarar(
	_ context.Context, simulacion string, ambito resolucionpublicado.Ambito, declaradas []resolucionpublicado.VariableDeclarada,
) error {
	v.simulacionRecibida, v.ambitoRecibido, v.declaradasRecibidas = simulacion, ambito, declaradas
	return v.err
}

func (v *variablesFalsas) Interpolar(
	_ context.Context, simulacion string, ambito resolucionpublicado.Ambito, _ string,
) (string, error) {
	v.simulacionRecibida, v.ambitoRecibido = simulacion, ambito
	return v.interpolado, v.err
}

func (v *variablesFalsas) RegistrarProducido(
	_ context.Context, simulacion, _, _ string, ambito resolucionpublicado.Ambito,
) error {
	v.simulacionRecibida, v.ambitoRecibido = simulacion, ambito
	return v.err
}

func (v *variablesFalsas) Cerrar(_ context.Context, simulacion string) error {
	v.simulacionRecibida = simulacion
	v.cerrada = true
	return v.err
}

func TestAdaptadorDeVariables_TraduceElAmbitoYLoDeclarado(t *testing.T) {
	falsas := &variablesFalsas{}
	adaptador := infraestructura.NuevasVariables(falsas)
	ambito, err := dominio.AmbitoDeAmbiente("prod")
	require.NoError(t, err)

	err = adaptador.Declarar(context.Background(), "sim-1", ambito, []dominio.VariableDeclarada{
		{Nombre: "replicas", Ambito: ambito, Valor: "5"},
	})
	require.NoError(t, err)
	require.Equal(t, "sim-1", falsas.simulacionRecibida)
	require.False(t, falsas.ambitoRecibido.Compartido)
	require.Equal(t, "prod", falsas.ambitoRecibido.Ambiente)
	require.Equal(t, []resolucionpublicado.VariableDeclarada{
		{Nombre: "replicas", Ambito: resolucionpublicado.Ambito{Ambiente: "prod"}, Valor: "5"},
	}, falsas.declaradasRecibidas)
}

func TestAdaptadorDeVariables_TraduceElAmbitoCompartido(t *testing.T) {
	falsas := &variablesFalsas{}
	adaptador := infraestructura.NuevasVariables(falsas)

	_, err := adaptador.Interpolar(context.Background(), "sim-1", dominio.AmbitoCompartido(), "texto")
	require.NoError(t, err)
	require.Equal(t, resolucionpublicado.Ambito{Compartido: true}, falsas.ambitoRecibido)
}

func TestAdaptadorDeVariables_ElNombreQueFaltaSigueSiendoRecuperableConErrorsAs(t *testing.T) {
	falsas := &variablesFalsas{err: &resolucionpublicado.VariableNoEncontradaError{Nombre: "no_declarada"}}
	adaptador := infraestructura.NuevasVariables(falsas)

	_, err := adaptador.Interpolar(context.Background(), "sim-1", dominio.AmbitoCompartido(), "${var.no_declarada}")
	var faltante *resolucionpublicado.VariableNoEncontradaError
	require.True(t, errors.As(err, &faltante), "el error envuelto sigue siendo recuperable")
	require.Equal(t, "no_declarada", faltante.Nombre)
}

func TestAdaptadorDeVariables_Cerrar(t *testing.T) {
	falsas := &variablesFalsas{}
	adaptador := infraestructura.NuevasVariables(falsas)

	require.NoError(t, adaptador.Cerrar(context.Background(), "sim-1"))
	require.True(t, falsas.cerrada)
	require.Equal(t, "sim-1", falsas.simulacionRecibida)
}
