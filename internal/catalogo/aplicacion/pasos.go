package aplicacion

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/publicado"
)

// Pasos son los del pipeline, en su orden.
func (s *Servicio) Pasos(ctx context.Context, p publicado.PeticionDeCatalogo) ([]publicado.Paso, error) {
	pipeline, err := s.pipeline(ctx, p)
	if err != nil {
		return nil, traducir(err)
	}
	pasos := make([]publicado.Paso, 0, len(pipeline.Pasos))
	for _, paso := range pipeline.Pasos {
		pasos = append(pasos, publicado.Paso{Nombre: paso.Nombre, Orden: paso.Orden, Compartido: paso.Compartido})
	}
	return pasos, nil
}
