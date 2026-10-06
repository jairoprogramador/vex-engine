package borde

import (
	"context"

	lanzamientopublicado "github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
)

// Lanzar comprueba la versión de la petición (DEC-05.6) y la delega en Lanzamiento (LAN-2).
func (s *Servicio) Lanzar(
	ctx context.Context, p PeticionDeLanzamiento,
) (lanzamientopublicado.Lanzamiento, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return lanzamientopublicado.Lanzamiento{}, err
	}
	return s.d.Lanzamiento.Lanzar(ctx, p.Ambiente, p.Despliegue, p.Nombre)
}

// Reservar comprueba la versión de la petición (DEC-05.6) y la delega en Lanzamiento (LAN-3).
func (s *Servicio) Reservar(ctx context.Context, p PeticionDeReserva) error {
	if err := comprobarVersion(p.Version); err != nil {
		return err
	}
	return s.d.Lanzamiento.Reservar(ctx, p.Ambiente)
}

// Liberar comprueba la versión de la petición (DEC-05.6) y la delega en Lanzamiento (LAN-3).
func (s *Servicio) Liberar(ctx context.Context, p PeticionDeLiberacion) error {
	if err := comprobarVersion(p.Version); err != nil {
		return err
	}
	return s.d.Lanzamiento.Liberar(ctx, p.Ambiente)
}
