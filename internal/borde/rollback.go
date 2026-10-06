package borde

import (
	"context"

	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// HacerRollback comprueba la versión del lenguaje publicado de la petición (DEC-05.6) y la delega en Ejecución
// de Pipeline — la operación hacer rollback a un destino (EJ-2).
func (s *Servicio) HacerRollback(
	ctx context.Context, p ejecucionpublicado.PeticionDeRollback, entorno ejecucionpublicado.Entorno,
) (ejecucionpublicado.Resultado, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return ejecucionpublicado.Resultado{}, err
	}
	return s.d.Ejecucion.HacerRollback(ctx, p, entorno)
}
