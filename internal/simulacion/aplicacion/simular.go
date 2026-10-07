package aplicacion

import (
	"context"
	"errors"
	"fmt"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// Simular es SIM-1: simula un intento hasta un paso de un ambiente, sin efectos. Trae el pipeline, de una copia
// de trabajo o de un commit (DEC-10.6), y lo comprueba — si la comprobación falla, esa es la causa (§1); si
// pasa, valida lo pedido contra él y recorre el ambiente hasta el paso (§2). El resultado es el resumen de lo
// que habría pasado (§3), con la causa si habría fallado.
//
// Pedir lo imposible — un ambiente o un paso que el pipeline no tiene — no es un intento que falla: es una
// petición inválida, y se devuelve como error.
func (s *Servicio) Simular(ctx context.Context, p publicado.PeticionDeSimulacion) (publicado.Resultado, error) {
	if err := comprobarPeticion(p); err != nil {
		return publicado.Resultado{}, traducir(err)
	}
	resultado := publicado.Resultado{
		Ambiente: p.Ambiente, Solicitante: p.Solicitante, HastaPaso: p.HastaPaso, Estado: publicado.EstadoExitoso,
	}

	pipeline, err := s.traerPipeline(ctx, p)
	if err != nil {
		var fallos *definicionpublicado.FallosDeComprobacion
		if errors.As(err, &fallos) {
			resultado.Estado = publicado.EstadoFallido
			resultado.Causa = &publicado.Causa{Fallos: traducirFallos(fallos.Fallos)}
			return resultado, nil
		}
		return publicado.Resultado{}, traducir(err)
	}

	ambiente, err := pipeline.AmbientePorValor(p.Ambiente)
	if err != nil {
		return publicado.Resultado{}, invalida(err)
	}
	pasos, err := pipeline.PasosHasta(p.HastaPaso)
	if err != nil {
		return publicado.Resultado{}, invalida(err)
	}

	faltante, err := s.recorrerAmbiente(ctx, pipeline, ambiente, pasos, p.Metadatos)
	if err != nil {
		return publicado.Resultado{}, traducir(err)
	}
	if faltante != nil {
		resultado.Estado = publicado.EstadoFallido
		resultado.Causa = &publicado.Causa{Faltante: []publicado.FaltanteDePaso{*faltante}}
	}
	return resultado, nil
}

// comprobarPeticion pide los tres campos obligatorios, siempre en el mismo orden: si faltan varios, quien invoca
// recibe siempre el mismo.
func comprobarPeticion(p publicado.PeticionDeSimulacion) error {
	obligatorios := []struct{ campo, valor string }{
		{"Ambiente", p.Ambiente}, {"Solicitante", p.Solicitante}, {"HastaPaso", p.HastaPaso},
	}
	for _, o := range obligatorios {
		if o.valor == "" {
			return dominio.NuevoParametroInvalido(o.campo, o.valor, "falta "+o.campo)
		}
	}
	return nil
}

func invalida(err error) error {
	return fmt.Errorf("%w: %w", publicado.ErrInvalido, err)
}

func (s *Servicio) traerPipeline(ctx context.Context, p publicado.PeticionDeSimulacion) (dominio.Pipeline, error) {
	if p.CopiaDeTrabajo != "" {
		return s.d.Pipelines.DeUnaCopiaDeTrabajo(ctx, p.CopiaDeTrabajo)
	}
	return s.d.Pipelines.DeUnCommit(ctx, p.Fuente, p.Commit)
}
