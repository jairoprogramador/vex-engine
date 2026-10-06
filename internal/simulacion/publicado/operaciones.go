package publicado

import "context"

// Lo que Simulación de Pipeline publica: simular un intento de un pipeline, sin efectos (docs/modelo/contextos/
// simulacion.md, «Servicio de aplicación»).

// ParaBorde es lo que usa el borde — su único cliente, igual que ejecucion/publicado.ParaBorde. Sin cliente
// real hasta RD-10.
type ParaBorde interface {
	Simular(ctx context.Context, p PeticionDeSimulacion) (Resultado, error)
}
