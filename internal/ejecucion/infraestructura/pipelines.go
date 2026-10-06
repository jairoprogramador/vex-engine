package infraestructura

import (
	"context"
	"fmt"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// Pipelines implementa dominio.Pipelines sobre lo que publica Definición de Pipeline.
type Pipelines struct {
	pipelines definicionpublicado.ParaEjecucion
}

var _ dominio.Pipelines = (*Pipelines)(nil)

func NuevosPipelines(p definicionpublicado.ParaEjecucion) *Pipelines {
	return &Pipelines{pipelines: p}
}

func (p *Pipelines) DeHoy(ctx context.Context, fuente string) (dominio.Pipeline, error) {
	pipeline, err := p.pipelines.DeHoy(ctx, fuente)
	if err != nil {
		return dominio.Pipeline{}, delContextoDeArriba(fmt.Errorf("ejecución: el pipeline de %q: %w", fuente, err))
	}
	return pipelineDeDominio(pipeline)
}

func (p *Pipelines) DeUnCommit(ctx context.Context, fuente, commit string) (dominio.Pipeline, error) {
	pipeline, err := p.pipelines.DeUnCommit(ctx, fuente, commit)
	if err != nil {
		return dominio.Pipeline{}, delContextoDeArriba(fmt.Errorf("ejecución: el pipeline de %s@%s: %w", fuente, commit, err))
	}
	return pipelineDeDominio(pipeline)
}

func (p *Pipelines) VariablesEstandar() []dominio.VariableEstandar {
	estandar := p.pipelines.VariablesEstandar()
	resultado := make([]dominio.VariableEstandar, 0, len(estandar))
	for _, e := range estandar {
		resultado = append(resultado, dominio.VariableEstandar{Nombre: e.Nombre, Metadato: e.Metadato, DelPaso: e.DelPaso})
	}
	return resultado
}

func pipelineDeDominio(p definicionpublicado.Pipeline) (dominio.Pipeline, error) {
	pasos := make([]dominio.PasoDeEjecucion, 0, len(p.Pasos))
	for _, paso := range p.Pasos {
		pasoDeEjecucion, err := pasoDeDominio(paso)
		if err != nil {
			return dominio.Pipeline{}, fmt.Errorf("ejecución: el paso %q: %w", paso.Nombre, err)
		}
		pasos = append(pasos, pasoDeEjecucion)
	}
	ambientes := make([]string, 0, len(p.Ambientes))
	for _, a := range p.Ambientes {
		ambientes = append(ambientes, a.Valor)
	}
	return dominio.Pipeline{Commit: p.Commit, Ambientes: ambientes, Pasos: pasos}, nil
}

func pasoDeDominio(paso definicionpublicado.Paso) (dominio.PasoDeEjecucion, error) {
	base, err := dominio.NuevoPasoDelPipeline(paso.Nombre, paso.Compartido)
	if err != nil {
		return dominio.PasoDeEjecucion{}, err
	}

	var miraCodigo, miraInstrucciones, miraVariables bool
	for _, r := range paso.Reglas {
		switch r {
		case definicionpublicado.ReglaCodigo:
			miraCodigo = true
		case definicionpublicado.ReglaInstrucciones:
			miraInstrucciones = true
		case definicionpublicado.ReglaVariables:
			miraVariables = true
		}
	}
	regla, err := dominio.NuevaRegla(miraCodigo, miraInstrucciones, miraVariables, paso.EdadMaxima)
	if err != nil {
		return dominio.PasoDeEjecucion{}, err
	}

	comandos := make([]dominio.ComandoDeclarado, 0, len(paso.Comandos))
	for _, c := range paso.Comandos {
		comando, err := comandoDeDominio(c)
		if err != nil {
			return dominio.PasoDeEjecucion{}, err
		}
		comandos = append(comandos, comando)
	}

	material := make([]dominio.FicheroDeclarado, 0, len(paso.Material))
	for _, f := range paso.Material {
		fichero, err := dominio.NuevoFicheroDeclarado(f.Ruta, f.Contenido, f.Ejecutable, f.Enlace, f.Plantilla)
		if err != nil {
			return dominio.PasoDeEjecucion{}, err
		}
		material = append(material, fichero)
	}

	return dominio.PasoDeEjecucion{PasoDelPipeline: base, Regla: regla, Comandos: comandos, Material: material}, nil
}

func comandoDeDominio(c definicionpublicado.Comando) (dominio.ComandoDeclarado, error) {
	salidas := make([]dominio.VariableDeSalidaDeclarada, 0, len(c.Salidas))
	for _, s := range c.Salidas {
		salida, err := dominio.NuevaVariableDeSalidaDeclarada(s.Nombre, s.Expresion, s.Compartida)
		if err != nil {
			return dominio.ComandoDeclarado{}, err
		}
		salidas = append(salidas, salida)
	}

	aserciones := make([]dominio.AsercionDeclarada, 0, len(c.Aserciones))
	for _, a := range c.Aserciones {
		asercion, err := dominio.NuevaAsercionDeclarada(a.Expresion)
		if err != nil {
			return dominio.ComandoDeclarado{}, err
		}
		aserciones = append(aserciones, asercion)
	}

	return dominio.NuevoComandoDeclarado(c.Nombre, c.Linea, c.Directorio, c.Plantillas, salidas, aserciones)
}
