package infraestructura_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/infraestructura"
)

// El adaptador de Pipelines se prueba contra un doble de definicion/publicado.ParaSimulacion (DEC-11.3).
type pipelinesFalsos struct {
	pipeline           definicionpublicado.Pipeline
	commitRecibido     string
	directorioRecibido string
	err                error
}

func (p *pipelinesFalsos) DeUnCommit(_ context.Context, _, commit string) (definicionpublicado.Pipeline, error) {
	p.commitRecibido = commit
	return p.pipeline, p.err
}

func (p *pipelinesFalsos) DeUnaCopiaDeTrabajo(_ context.Context, directorio string) (definicionpublicado.Pipeline, error) {
	p.directorioRecibido = directorio
	return p.pipeline, p.err
}

func (p *pipelinesFalsos) VariablesEstandar() []definicionpublicado.VariableEstandar { return nil }

func pipelineDeEjemplo() definicionpublicado.Pipeline {
	return definicionpublicado.Pipeline{
		Ambientes: []definicionpublicado.Ambiente{{Nombre: "sandbox", Valor: "sand"}},
		Pasos: []definicionpublicado.Paso{{
			Nombre:     "registro",
			Compartido: true,
			Comandos: []definicionpublicado.Comando{{
				Nombre: "crear", Linea: "crear ${var.sku}",
				Salidas: []definicionpublicado.VariableDeSalida{{Nombre: "registro", Expresion: "(.+)", Compartida: true}},
			}},
			Material: []definicionpublicado.Fichero{{Ruta: "k8s/d.yaml", Contenido: "image: ${var.imagen}", Plantilla: true}},
		}},
		Variables: []definicionpublicado.VariableDeclarada{
			{Nombre: "sku", Valor: "Basic"},
			{Nombre: "imagen", Ambito: "sand", Valor: "app:1"},
		},
	}
}

func TestAdaptadorDePipelines_TraduceElPipelineDeUnCommit(t *testing.T) {
	falso := &pipelinesFalsos{pipeline: pipelineDeEjemplo()}
	adaptador := infraestructura.NuevosPipelines(falso)

	pipeline, err := adaptador.DeUnCommit(context.Background(), "fuente", "c1")
	require.NoError(t, err)
	require.Equal(t, "c1", falso.commitRecibido)
	require.Len(t, pipeline.Ambientes, 1)
	require.Equal(t, "sand", pipeline.Ambientes[0].Valor)
	require.Len(t, pipeline.Pasos, 1)
	require.True(t, pipeline.Pasos[0].Compartido)
	require.Equal(t, "crear ${var.sku}", pipeline.Pasos[0].Comandos[0].Linea)
	require.Equal(t,
		[]dominio.VariableDeSalida{{Nombre: "registro", Expresion: "(.+)", Compartida: true}},
		pipeline.Pasos[0].Comandos[0].Salidas,
	)
	require.True(t, pipeline.Pasos[0].Material[0].Plantilla)
	require.Len(t, pipeline.Variables, 2)
	require.True(t, pipeline.Variables[0].Ambito.EsCompartido(), "sin ámbito de Definición es el ámbito compartido")
	require.Equal(t, "sand", pipeline.Variables[1].Ambito.Ambiente())
}

func TestAdaptadorDePipelines_TraduceElPipelineDeUnaCopiaDeTrabajo(t *testing.T) {
	falso := &pipelinesFalsos{pipeline: pipelineDeEjemplo()}
	adaptador := infraestructura.NuevosPipelines(falso)

	_, err := adaptador.DeUnaCopiaDeTrabajo(context.Background(), "/mi/copia")
	require.NoError(t, err)
	require.Equal(t, "/mi/copia", falso.directorioRecibido)
}

func TestAdaptadorDePipelines_PropagaElError(t *testing.T) {
	causa := errors.New("no existe")
	falso := &pipelinesFalsos{err: causa}
	adaptador := infraestructura.NuevosPipelines(falso)

	_, err := adaptador.DeUnCommit(context.Background(), "fuente", "c1")
	require.ErrorIs(t, err, causa)
}
