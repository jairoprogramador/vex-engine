package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
)

// LanzamientosDeUnAmbiente es la consulta de los lanzamientos de un ambiente, del más antiguo al más reciente.
func (s *Servicio) LanzamientosDeUnAmbiente(ctx context.Context, ambiente string) ([]publicado.Lanzamiento, error) {
	a, err := dominio.NuevaAmbiente(ambiente)
	if err != nil {
		return nil, traducir(err)
	}
	registrados, err := s.d.Historial.LanzamientosDeUnAmbiente(ctx, a)
	if err != nil {
		return nil, traducir(err)
	}
	lanzamientos := make([]publicado.Lanzamiento, 0, len(registrados))
	for _, r := range registrados {
		lanzamientos = append(lanzamientos, lanzamientoAPublicado(r))
	}
	return lanzamientos, nil
}
