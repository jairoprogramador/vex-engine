package borde

import (
	"context"

	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Lo que el borde deja consultar del Historial nunca lleva el valor de una variable (DEC-04.7): los tipos de
// historialpublicado no tienen dónde ponerlo, y el valor solo sale por la relación reservada hacia Resolución.

// AbandonarIntento comprueba la versión de la petición (DEC-05.6) y da por abandonado un intento sin
// desenlace en el Historial, liberando su ambiente (DEC-07.8).
func (s *Servicio) AbandonarIntento(ctx context.Context, p PeticionDeAbandono) error {
	if err := comprobarVersion(p.Version); err != nil {
		return err
	}
	return s.d.Historial.AbandonarIntento(ctx, p.Intento)
}

// Intento comprueba la versión de la petición (DEC-05.6) y consulta un intento por su identidad.
func (s *Servicio) Intento(
	ctx context.Context, p PeticionDeConsultaDeIntento,
) (historialpublicado.Intento, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return historialpublicado.Intento{}, err
	}
	return s.d.Historial.Intento(ctx, p.Intento)
}

// IntentosDeUnAmbiente comprueba la versión de la petición (DEC-05.6) y consulta los intentos de un ambiente.
func (s *Servicio) IntentosDeUnAmbiente(
	ctx context.Context, p PeticionDeIntentosDeUnAmbiente,
) ([]historialpublicado.Intento, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return nil, err
	}
	return s.d.Historial.IntentosDeUnAmbiente(ctx, p.Ambiente)
}

// DesplieguesDeUnAmbiente comprueba la versión de la petición (DEC-05.6) y consulta los despliegues de un
// ambiente.
func (s *Servicio) DesplieguesDeUnAmbiente(
	ctx context.Context, p PeticionDeDesplieguesDeUnAmbiente,
) ([]historialpublicado.Despliegue, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return nil, err
	}
	return s.d.Historial.DesplieguesDeUnAmbiente(ctx, p.Ambiente)
}
