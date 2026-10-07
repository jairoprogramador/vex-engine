package infraestructura

import (
	"context"
	"fmt"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

// Pipelines implementa dominio.Pipelines sobre lo que publica Definición de Pipeline.
type Pipelines struct {
	pipelines definicionpublicado.ParaSimulacion
}

var _ dominio.Pipelines = (*Pipelines)(nil)

func NuevosPipelines(p definicionpublicado.ParaSimulacion) *Pipelines {
	return &Pipelines{pipelines: p}
}

func (p *Pipelines) DeUnCommit(ctx context.Context, fuente, commit string) (dominio.Pipeline, error) {
	pipeline, err := p.pipelines.DeUnCommit(ctx, fuente, commit)
	if err != nil {
		return dominio.Pipeline{}, delContextoDeArriba(fmt.Errorf("simulación: el pipeline de %s@%s: %w", fuente, commit, err))
	}
	return pipelineDeDominio(pipeline)
}

func (p *Pipelines) DeUnaCopiaDeTrabajo(ctx context.Context, directorio string) (dominio.Pipeline, error) {
	pipeline, err := p.pipelines.DeUnaCopiaDeTrabajo(ctx, directorio)
	if err != nil {
		return dominio.Pipeline{}, delContextoDeArribaEnElCampo(
			fmt.Errorf("simulación: el pipeline de la copia de trabajo %q: %w", directorio, err), "CopiaDeTrabajo", directorio)
	}
	return pipelineDeDominio(pipeline)
}

func (p *Pipelines) VariablesEstandar() []dominio.VariableEstandar {
	estandar := p.pipelines.VariablesEstandar()
	resultado := make([]dominio.VariableEstandar, 0, len(estandar))
	for _, e := range estandar {
		resultado = append(resultado, dominio.VariableEstandar{Nombre: e.Nombre, DelPaso: e.DelPaso})
	}
	return resultado
}

func pipelineDeDominio(p definicionpublicado.Pipeline) (dominio.Pipeline, error) {
	ambientes := make([]dominio.Ambiente, 0, len(p.Ambientes))
	for _, a := range p.Ambientes {
		ambientes = append(ambientes, dominio.Ambiente{Nombre: a.Nombre, Valor: a.Valor})
	}
	pasos := make([]dominio.Paso, 0, len(p.Pasos))
	for _, paso := range p.Pasos {
		pasos = append(pasos, pasoDeDominio(paso))
	}
	variables := make([]dominio.VariableDeclarada, 0, len(p.Variables))
	for _, v := range p.Variables {
		variable, err := variableDeclaradaDeDominio(v)
		if err != nil {
			return dominio.Pipeline{}, fmt.Errorf("simulación: la variable %q: %w", v.Nombre, err)
		}
		variables = append(variables, variable)
	}
	return dominio.Pipeline{Ambientes: ambientes, Pasos: pasos, Variables: variables}, nil
}

func pasoDeDominio(paso definicionpublicado.Paso) dominio.Paso {
	comandos := make([]dominio.Comando, 0, len(paso.Comandos))
	for _, c := range paso.Comandos {
		comandos = append(comandos, comandoDeDominio(c))
	}
	material := make([]dominio.Fichero, 0, len(paso.Material))
	for _, f := range paso.Material {
		material = append(material, dominio.Fichero{Ruta: f.Ruta, Contenido: f.Contenido, Plantilla: f.Plantilla})
	}
	return dominio.Paso{Nombre: paso.Nombre, Compartido: paso.Compartido, Comandos: comandos, Material: material}
}

func comandoDeDominio(c definicionpublicado.Comando) dominio.Comando {
	salidas := make([]dominio.VariableDeSalida, 0, len(c.Salidas))
	for _, s := range c.Salidas {
		salidas = append(salidas, dominio.VariableDeSalida{Nombre: s.Nombre, Expresion: s.Expresion, Compartida: s.Compartida})
	}
	return dominio.Comando{Nombre: c.Nombre, Linea: c.Linea, Salidas: salidas}
}

// variableDeclaradaDeDominio traduce el ámbito como texto de Definición (vacío = compartido) al Ambito
// tipado de este dominio.
func variableDeclaradaDeDominio(v definicionpublicado.VariableDeclarada) (dominio.VariableDeclarada, error) {
	if v.Ambito == "" {
		return dominio.VariableDeclarada{Nombre: v.Nombre, Ambito: dominio.AmbitoCompartido(), Valor: v.Valor}, nil
	}
	ambito, err := dominio.AmbitoDeAmbiente(v.Ambito)
	if err != nil {
		return dominio.VariableDeclarada{}, err
	}
	return dominio.VariableDeclarada{Nombre: v.Nombre, Ambito: ambito, Valor: v.Valor}, nil
}
