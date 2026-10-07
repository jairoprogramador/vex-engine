package infraestructura

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/dominio"
	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
)

// Pipelines implementa dominio.Pipelines sobre lo que publica Definición de Pipeline.
type Pipelines struct {
	pipelines definicionpublicado.ParaEjecucion
}

var _ dominio.Pipelines = (*Pipelines)(nil)

func NuevosPipelines(p definicionpublicado.ParaEjecucion) *Pipelines {
	return &Pipelines{pipelines: p}
}

func (p *Pipelines) Pipeline(ctx context.Context, fuente dominio.Fuente, commit dominio.Commit) (dominio.Pipeline, error) {
	var pipeline definicionpublicado.Pipeline
	var err error
	if commit.DeHoy() {
		pipeline, err = p.pipelines.DeHoy(ctx, fuente.String())
	} else {
		pipeline, err = p.pipelines.DeUnCommit(ctx, fuente.String(), commit.String())
	}
	if err != nil {
		return dominio.Pipeline{}, delContextoDeArriba(fmt.Errorf("catálogo: el pipeline de %q: %w", fuente, err))
	}
	return pipelineDeDominio(pipeline), nil
}

func pipelineDeDominio(p definicionpublicado.Pipeline) dominio.Pipeline {
	ambientes := make([]dominio.Ambiente, 0, len(p.Ambientes))
	for _, a := range p.Ambientes {
		ambientes = append(ambientes, dominio.Ambiente{Nombre: a.Nombre, Descripcion: a.Descripcion, Valor: a.Valor})
	}
	pasos := make([]dominio.Paso, 0, len(p.Pasos))
	for _, paso := range p.Pasos {
		pasos = append(pasos, dominio.Paso{Nombre: paso.Nombre, Orden: paso.Orden, Compartido: paso.Compartido})
	}
	return dominio.Pipeline{Ambientes: ambientes, Pasos: pasos}
}
