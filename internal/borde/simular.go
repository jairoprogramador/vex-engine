package borde

import (
	"context"

	simulacionpublicado "github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// Simular comprueba la versión de la petición (DEC-05.6) y la delega en Simulación de Pipeline (SIM-1).
func (s *Servicio) Simular(
	ctx context.Context, p simulacionpublicado.PeticionDeSimulacion,
) (simulacionpublicado.Resultado, error) {
	if err := comprobarVersion(p.Version); err != nil {
		return simulacionpublicado.Resultado{}, err
	}
	return s.d.Simulacion.Simular(ctx, p)
}
