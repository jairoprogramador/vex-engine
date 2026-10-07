package borde

import (
	"context"

	catalogopublicado "github.com/jairoprogramador/vex-engine/internal/catalogo/publicado"
)

// Ambientes comprueba la versión de la petición (DEC-05.6) y los pide a Catálogo, con su estado de reserva.
func (s *Servicio) Ambientes(
	ctx context.Context, p catalogopublicado.PeticionDeCatalogo,
) ([]catalogopublicado.Ambiente, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return nil, err
	}
	return s.d.Catalogo.Ambientes(ctx, p)
}

// Pasos comprueba la versión de la petición (DEC-05.6) y los pide a Catálogo.
func (s *Servicio) Pasos(
	ctx context.Context, p catalogopublicado.PeticionDeCatalogo,
) ([]catalogopublicado.Paso, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return nil, err
	}
	return s.d.Catalogo.Pasos(ctx, p)
}
