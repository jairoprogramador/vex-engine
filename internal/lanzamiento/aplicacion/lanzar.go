package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
)

// Lanzar es LAN-2: el dueño del negocio lanza un despliegue, con nombre o sin él. Es incondicional — la
// reserva de un ambiente solo bloquea el lanzamiento en nombre del actor ausente (IT-04 DEC-04.1).
func (s *Servicio) Lanzar(ctx context.Context, ambiente, despliegue, nombre string) (publicado.Lanzamiento, error) {
	a, err := dominio.NuevaAmbiente(ambiente)
	if err != nil {
		return publicado.Lanzamiento{}, traducir(err)
	}
	d, err := dominio.NuevoIdDespliegue(despliegue)
	if err != nil {
		return publicado.Lanzamiento{}, traducir(err)
	}
	registrado, err := s.lanzar(ctx, a, d, dominio.Nombre(nombre))
	if err != nil {
		return publicado.Lanzamiento{}, traducir(err)
	}
	return lanzamientoAPublicado(registrado), nil
}

// lanzar decide la versión y registra el lanzamiento. Lo comparten Lanzar (LAN-2) y AlRegistrarseUnDespliegue
// (LAN-1, con nombre vacío).
func (s *Servicio) lanzar(
	ctx context.Context, ambiente dominio.Ambiente, despliegue dominio.IdDespliegue, nombre dominio.Nombre,
) (dominio.LanzamientoRegistrado, error) {
	hash, err := s.d.Historial.HashDelCodigoDeUnDespliegue(ctx, despliegue)
	if err != nil {
		return dominio.LanzamientoRegistrado{}, err
	}
	conocidas, err := s.d.Historial.VersionesConocidas(ctx)
	if err != nil {
		return dominio.LanzamientoRegistrado{}, err
	}
	version := dominio.DecidirVersion(hash, conocidas)
	lanzamiento := dominio.NuevoLanzamiento(despliegue, hash, version, nombre)
	return s.d.Historial.RegistrarLanzamiento(ctx, ambiente, lanzamiento)
}
