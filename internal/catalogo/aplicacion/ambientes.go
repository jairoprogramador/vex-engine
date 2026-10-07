package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/dominio"
	"github.com/jairoprogramador/vex-engine/internal/catalogo/publicado"
)

// Ambientes son los del pipeline, en su orden, cada uno con su estado de reserva.
func (s *Servicio) Ambientes(ctx context.Context, p publicado.PeticionDeCatalogo) ([]publicado.Ambiente, error) {
	pipeline, err := s.pipeline(ctx, p)
	if err != nil {
		return nil, traducir(err)
	}
	ambientes := make([]publicado.Ambiente, 0, len(pipeline.Ambientes))
	for _, a := range pipeline.Ambientes {
		reservado, err := s.d.Reservas.Reservado(ctx, a.Valor)
		if err != nil {
			return nil, traducir(err)
		}
		ambientes = append(ambientes, ambienteAPublicado(dominio.AmbienteReservable{Ambiente: a, Reservado: reservado}))
	}
	return ambientes, nil
}

func ambienteAPublicado(a dominio.AmbienteReservable) publicado.Ambiente {
	return publicado.Ambiente{Nombre: a.Nombre, Descripcion: a.Descripcion, Valor: a.Valor, Reservado: a.Reservado}
}
