package aplicacion

import (
	"context"
	"errors"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// Simular es SIM-1: trae el pipeline, de una copia de trabajo o de un commit (DEC-10.6), y lo comprueba — si
// la comprobación falla, ese es el resultado entero (§1); si pasa, recorre todos los ambientes (§2) y entrega
// el informe (§3).
func (s *Servicio) Simular(ctx context.Context, p publicado.PeticionDeSimulacion) (publicado.Informe, error) {
	pipeline, err := s.traerPipeline(ctx, p)
	if err != nil {
		var fallos *definicionpublicado.FallosDeComprobacion
		if errors.As(err, &fallos) {
			return publicado.Informe{
				Comprobacion: publicado.ResultadoDeComprobacion{Paso: false, Fallos: traducirFallos(fallos.Fallos)},
			}, nil
		}
		return publicado.Informe{}, err
	}

	ambientes, err := s.recorrerAmbientes(ctx, pipeline)
	if err != nil {
		return publicado.Informe{}, err
	}
	return publicado.Informe{
		Comprobacion: publicado.ResultadoDeComprobacion{Paso: true},
		Ambientes:    ambientes,
	}, nil
}

func (s *Servicio) traerPipeline(ctx context.Context, p publicado.PeticionDeSimulacion) (dominio.Pipeline, error) {
	if p.CopiaDeTrabajo != "" {
		return s.d.Pipelines.DeUnaCopiaDeTrabajo(ctx, p.CopiaDeTrabajo)
	}
	return s.d.Pipelines.DeUnCommit(ctx, p.Fuente, p.Commit)
}
