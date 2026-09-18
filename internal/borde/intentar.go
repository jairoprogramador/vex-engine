package borde

import (
	"context"

	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// Intentar comprueba la versión del lenguaje publicado de la petición (DEC-05.6) y la delega en Ejecución de
// Pipeline — la operación intentar hasta un paso en un ambiente (EJ-1).
func (s *Servicio) Intentar(
	ctx context.Context, p ejecucionpublicado.PeticionDeIntento, salida ejecucionpublicado.Salida,
) (ejecucionpublicado.Resultado, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return ejecucionpublicado.Resultado{}, err
	}
	return s.d.Ejecucion.Intentar(ctx, p, salida)
}
