package infraestructura

import (
	"context"
	"errors"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/definicion/dominio"
	suministro "github.com/jairoprogramador/vex-engine/internal/suministro/publicado"
)

// PipelinesDeSuministro es el ACL hacia Suministro de Fuentes (context-map.md, fila #12): trae la fuente del
// pipeline, la convierte en una declaración, la comprueba y retira el material. El pipeline sale entero en
// memoria, material incluido, así que nadie más depende de que la copia siga ahí.
type PipelinesDeSuministro struct {
	suministro suministro.ParaDefinicion
}

var _ dominio.Pipelines = (*PipelinesDeSuministro)(nil)

func NuevosPipelinesDeSuministro(s suministro.ParaDefinicion) *PipelinesDeSuministro {
	return &PipelinesDeSuministro{suministro: s}
}

func (p *PipelinesDeSuministro) DeHoy(ctx context.Context, fuente string) (*dominio.PipelineComprobado, error) {
	material, err := p.suministro.TraerDeHoy(ctx, fuente)
	if err != nil {
		return nil, traducir(err)
	}
	return p.comprobar(ctx, material)
}

func (p *PipelinesDeSuministro) DeUnCommit(ctx context.Context, fuente, commit string) (*dominio.PipelineComprobado, error) {
	material, err := p.suministro.TraerDeUnCommit(ctx, fuente, commit)
	if err != nil {
		return nil, traducir(err)
	}
	return p.comprobar(ctx, material)
}

// comprobar lee el material, lo comprueba y siempre lo retira. Si no se puede retirar, no se entrega el
// pipeline: el error no se pierde.
func (p *PipelinesDeSuministro) comprobar(ctx context.Context, material suministro.Material) (*dominio.PipelineComprobado, error) {
	declaracion, err := leer(material.Directorio)
	var pipeline *dominio.PipelineComprobado
	if err == nil {
		declaracion.Commit = material.Commit
		declaracion.Hash = material.Hash
		pipeline, err = dominio.Comprobar(declaracion)
	}
	if errRetirar := p.suministro.Retirar(context.WithoutCancel(ctx), material); errRetirar != nil {
		return nil, errors.Join(err, fmt.Errorf("definicion: retirar el material del pipeline: %w", errRetirar))
	}
	return pipeline, err
}

// traducir lleva los errores de Suministro a los del dominio, sin perder su mensaje.
func traducir(err error) error {
	switch {
	case errors.Is(err, suministro.ErrNoExiste):
		return fmt.Errorf("%w: %w", dominio.ErrNoExiste, err)
	case errors.Is(err, suministro.ErrInvalido):
		return fmt.Errorf("%w: %w", dominio.ErrInvalido, err)
	}
	return fmt.Errorf("definicion: traer el pipeline: %w", err)
}
