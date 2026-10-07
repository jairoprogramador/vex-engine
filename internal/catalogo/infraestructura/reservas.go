package infraestructura

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/dominio"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Reservas implementa dominio.Reservas sobre lo que publica el Historial.
type Reservas struct {
	registros historialpublicado.ParaCatalogo
}

var _ dominio.Reservas = (*Reservas)(nil)

func NuevasReservas(registros historialpublicado.ParaCatalogo) *Reservas {
	return &Reservas{registros: registros}
}

func (r *Reservas) Reservado(ctx context.Context, ambiente string) (bool, error) {
	reserva, hay, err := r.registros.UltimaReserva(ctx, ambiente)
	if err != nil {
		return false, fmt.Errorf("catálogo: la última reserva de %q: %w", ambiente, err)
	}
	return hay && reserva.Reservado, nil
}
