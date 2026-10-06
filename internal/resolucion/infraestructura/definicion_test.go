package infraestructura_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/resolucion/infraestructura"
)

// pipelinesFalsos es el doble de definicion/publicado.ParaResolucion: la frontera que este adaptador
// traduce, no Definición misma.
type pipelinesFalsos struct {
	fuente, commit string
	pipeline       definicionpublicado.Pipeline
	estandar       []definicionpublicado.VariableEstandar
}

func (p *pipelinesFalsos) DeUnCommit(_ context.Context, fuente, commit string) (definicionpublicado.Pipeline, error) {
	if fuente != p.fuente || commit != p.commit {
		return definicionpublicado.Pipeline{}, context.DeadlineExceeded
	}
	return p.pipeline, nil
}

func (p *pipelinesFalsos) VariablesEstandar() []definicionpublicado.VariableEstandar {
	return p.estandar
}

var _ definicionpublicado.ParaResolucion = (*pipelinesFalsos)(nil)

func TestAdaptadorDeDefinicion_TraeLasVariablesDelCommitPedido(t *testing.T) {
	pipelines := &pipelinesFalsos{
		fuente: "https://github.com/vexja/pipeline", commit: "c1",
		pipeline: definicionpublicado.Pipeline{
			Variables: []definicionpublicado.VariableDeclarada{
				{Nombre: "host", Ambito: "prod", Valor: "vexja.com"},
				{Nombre: "compartida", Ambito: "", Valor: "v"},
			},
		},
		estandar: []definicionpublicado.VariableEstandar{{Nombre: "ejecucion_id", Metadato: true}},
	}
	adaptador := infraestructura.NuevaDefinicion(pipelines)

	declaradas, err := adaptador.VariablesDeclaradas(context.Background(), "https://github.com/vexja/pipeline", "c1")
	require.NoError(t, err)
	require.Len(t, declaradas, 2)

	prod, err := dominio.AmbitoDeAmbiente("prod")
	require.NoError(t, err)
	require.Contains(t, declaradas, dominio.VariableDeclarada{Nombre: "host", Ambito: prod, Valor: "vexja.com"})
	require.Contains(t, declaradas, dominio.VariableDeclarada{Nombre: "compartida", Ambito: dominio.AmbitoCompartido(), Valor: "v"})

	estandar := adaptador.VariablesEstandar()
	require.Equal(t, []dominio.VariableEstandar{{Nombre: "ejecucion_id", Metadato: true}}, estandar)

	_, err = adaptador.VariablesDeclaradas(context.Background(), "otra-fuente", "c1")
	require.Error(t, err)
}
