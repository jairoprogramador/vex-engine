package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

// Reservar y Liberar son LAN-3. Dos métodos, no un único método con un booleano: cada uno nombra la
// decisión de negocio que registra, sin que quien llama tenga que traducir un flag.

func (s *Servicio) Reservar(ctx context.Context, ambiente string) error {
	return s.registrarReserva(ctx, ambiente, true)
}

func (s *Servicio) Liberar(ctx context.Context, ambiente string) error {
	return s.registrarReserva(ctx, ambiente, false)
}

func (s *Servicio) registrarReserva(ctx context.Context, ambiente string, reservado bool) error {
	a, err := dominio.NuevaAmbiente(ambiente)
	if err != nil {
		return traducir(err)
	}
	if err := s.d.Historial.RegistrarReserva(ctx, a, reservado); err != nil {
		return traducir(err)
	}
	return nil
}
