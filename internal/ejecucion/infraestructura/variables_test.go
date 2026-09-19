package infraestructura_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
)

// El adaptador de Variables se prueba contra un doble de resolucion/publicado.ParaEjecucion (DEC-11.3): la
// interacción real con Resolución se ejercita en la prueba de punta a punta.
type variablesFalsas struct {
	ambitoRecibido   resolucionpublicado.Ambito
	estandarRecibido map[string]string
	interpolado      string
	hash             string
	err              error
}

func (v *variablesFalsas) VariablesDeUnPaso(
	_ context.Context, _, _ string, ambito resolucionpublicado.Ambito, _, _ string, estandar map[string]string,
) ([]resolucionpublicado.Variable, error) {
	v.ambitoRecibido = ambito
	v.estandarRecibido = estandar
	return nil, v.err
}

func (v *variablesFalsas) Interpolar(
	_ context.Context, _, _ string, ambito resolucionpublicado.Ambito, _ string,
) (string, error) {
	v.ambitoRecibido = ambito
	return v.interpolado, v.err
}

func (v *variablesFalsas) HashDeLasVariables(
	_ context.Context, _ string, _ resolucionpublicado.Ambito,
) (string, error) {
	return v.hash, v.err
}

func (v *variablesFalsas) RegistrarProducido(
	_ context.Context, _, _, _, _ string, _ resolucionpublicado.Ambito,
) error {
	return v.err
}

func (v *variablesFalsas) NoReejecutado(_ context.Context, _, _ string, _ resolucionpublicado.Ambito) error {
	return v.err
}

func TestAdaptadorDeVariables_TraduceElAmbitoDeAmbienteYElPuenteEstandar(t *testing.T) {
	falsas := &variablesFalsas{}
	adaptador := infraestructura.NuevasVariables(falsas)
	ambito, err := dominio.AmbitoDeAmbiente("prod")
	require.NoError(t, err)
	estandar := map[string]string{"project_id": "1"}

	err = adaptador.DeclararVariablesDeUnPaso(context.Background(), "int-1", "01-pruebas", ambito, "fuente", "c1", estandar)
	require.NoError(t, err)
	require.False(t, falsas.ambitoRecibido.Compartido)
	require.Equal(t, "prod", falsas.ambitoRecibido.Ambiente)
	require.Equal(t, estandar, falsas.estandarRecibido)
}

func TestAdaptadorDeVariables_TraduceElAmbitoCompartido(t *testing.T) {
	falsas := &variablesFalsas{}
	adaptador := infraestructura.NuevasVariables(falsas)

	_, err := adaptador.Interpolar(context.Background(), "int-1", "01-pruebas", dominio.AmbitoCompartido(), "texto")
	require.NoError(t, err)
	require.Equal(t, resolucionpublicado.Ambito{Compartido: true}, falsas.ambitoRecibido)
}

func TestAdaptadorDeVariables_PropagaElHashDeLasVariables(t *testing.T) {
	falsas := &variablesFalsas{hash: "entradas-v1:abc"}
	adaptador := infraestructura.NuevasVariables(falsas)

	hash, err := adaptador.HashDeLasVariables(context.Background(), "int-1", dominio.AmbitoCompartido())
	require.NoError(t, err)
	require.Equal(t, dominio.NuevoHashDeVariables("entradas-v1:abc"), hash)
}
