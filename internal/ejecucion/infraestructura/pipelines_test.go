package infraestructura_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/infraestructura"
)

// El adaptador de Pipelines se prueba contra un doble de definicion/publicado.ParaEjecucion: la frontera que
// Ejecución ya decidió (DEC-11.3). El pipeline real, leído de disco por Definición, se ejercita en la prueba de
// punta a punta.
type pipelinesFalsos struct {
	pipeline definicionpublicado.Pipeline
	estandar []definicionpublicado.VariableEstandar
	err      error
}

func (p *pipelinesFalsos) DeHoy(_ context.Context, _ string) (definicionpublicado.Pipeline, error) {
	return p.pipeline, p.err
}

func (p *pipelinesFalsos) DeUnCommit(_ context.Context, _, _ string) (definicionpublicado.Pipeline, error) {
	return p.pipeline, p.err
}

func (p *pipelinesFalsos) VariablesEstandar() []definicionpublicado.VariableEstandar {
	return p.estandar
}

func TestAdaptadorDePipelines_TraduceLosPasosConSuReglaSusComandosYSuMaterial(t *testing.T) {
	falsos := &pipelinesFalsos{
		pipeline: definicionpublicado.Pipeline{
			Commit: "c1",
			Pasos: []definicionpublicado.Paso{{
				Nombre:     "01-pruebas",
				Compartido: true,
				Reglas:     []definicionpublicado.Regla{definicionpublicado.ReglaCodigo, definicionpublicado.ReglaVariables},
				EdadMaxima: 30 * time.Minute,
				Comandos: []definicionpublicado.Comando{{
					Nombre: "test", Linea: "go test ./...", Directorio: "",
					Plantillas: []string{"a.tpl"},
					Salidas:    []definicionpublicado.VariableDeSalida{{Nombre: "tag", Expresion: "v(.+)", Compartida: true}},
					Aserciones: []definicionpublicado.Asercion{{Expresion: "OK"}},
				}},
				Material: []definicionpublicado.Fichero{{Ruta: "a.tpl", Contenido: "hola", Plantilla: true}},
			}},
		},
	}
	adaptador := infraestructura.NuevosPipelines(falsos)

	pipeline, err := adaptador.DeHoy(context.Background(), "fuente")
	require.NoError(t, err)
	require.Equal(t, "c1", pipeline.Commit)
	require.Len(t, pipeline.Pasos, 1)

	paso := pipeline.Pasos[0]
	require.Equal(t, "01-pruebas", paso.Nombre())
	require.True(t, paso.Compartido())
	require.True(t, paso.Regla.MiraCodigo())
	require.False(t, paso.Regla.MiraInstrucciones())
	require.True(t, paso.Regla.MiraVariables())
	require.Equal(t, 30*time.Minute, paso.Regla.EdadMaxima())
	require.Len(t, paso.Comandos, 1)
	require.Equal(t, "go test ./...", paso.Comandos[0].Linea())
	require.Len(t, paso.Comandos[0].Salidas(), 1)
	require.Len(t, paso.Comandos[0].Aserciones(), 1)
	require.Len(t, paso.Material, 1)
	require.True(t, paso.Material[0].Plantilla())
}

func TestAdaptadorDePipelines_TraduceLasVariablesEstandar(t *testing.T) {
	falsos := &pipelinesFalsos{
		estandar: []definicionpublicado.VariableEstandar{
			{Nombre: "project_id", Metadato: true},
			{Nombre: "step_name", DelPaso: true},
		},
	}
	adaptador := infraestructura.NuevosPipelines(falsos)

	estandar := adaptador.VariablesEstandar()
	require.Len(t, estandar, 2)
	require.Equal(t, "project_id", estandar[0].Nombre)
	require.True(t, estandar[0].Metadato)
	require.Equal(t, "step_name", estandar[1].Nombre)
	require.True(t, estandar[1].DelPaso)
}

func TestAdaptadorDePipelines_PropagaElErrorDeDefinicion(t *testing.T) {
	falsos := &pipelinesFalsos{err: definicionpublicado.ErrNoExiste}
	adaptador := infraestructura.NuevosPipelines(falsos)

	_, err := adaptador.DeHoy(context.Background(), "fuente")
	require.ErrorIs(t, err, definicionpublicado.ErrNoExiste)
}
