package borde

import (
	"context"

	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

// PreguntarLaCausa comprueba la versión de la petición (DEC-05.6) y la delega en Diagnóstico.
func (s *Servicio) PreguntarLaCausa(
	ctx context.Context, p diagnosticopublicado.PeticionDeDiagnostico,
) (diagnosticopublicado.Respuesta, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return diagnosticopublicado.Respuesta{}, err
	}
	return s.d.Diagnostico.PreguntarLaCausa(ctx, p)
}
