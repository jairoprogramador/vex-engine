package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
)

// AlRegistrarseUnDespliegue es LAN-1: se pone al día cada vez que el Historial anuncia un despliegue
// registrado, y decide si Lanzamiento actúa en nombre del actor ausente (IT-04 DEC-04.8). No implementa la
// interfaz publicada del Historial: infraestructura/escucha.go traduce el evento a esta firma, en cadenas
// simples, para que este paquete nunca conozca el lenguaje del Historial.
func (s *Servicio) AlRegistrarseUnDespliegue(ctx context.Context, ambiente, despliegue string) error {
	a, err := dominio.NuevaAmbiente(ambiente)
	if err != nil {
		return err
	}
	d, err := dominio.NuevoIdDespliegue(despliegue)
	if err != nil {
		return err
	}
	reservado, err := s.d.Historial.Reservado(ctx, a)
	if err != nil {
		return err
	}
	ultimo, hay, err := s.d.Historial.UltimoLanzamientoDeUnAmbiente(ctx, a)
	if err != nil {
		return err
	}
	lanza := dominio.DecidirSiLanzarEnNombreDelActorAusente(dominio.ParametrosDecisionDeLanzamiento{
		Reservado: reservado, HayUltimoLanzamiento: hay,
		DespliegueDelUltimoLanzamiento: ultimo, DespliegueNuevo: d,
	})
	if !lanza {
		return nil
	}
	_, err = s.lanzar(ctx, a, d, "")
	return err
}
