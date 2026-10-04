package publicado

import "context"

// Lo que Ejecución de Pipeline publica: intentar hasta un paso en un ambiente, y hacer rollback a un destino
// (docs/modelo/contextos/ejecucion.md, «Lo que publica»). La salida de los comandos no sale por aquí: se
// guarda en el Historial y se consulta de allí.

// ParaBorde es lo que usa el borde mínimo (RD-06) — su único cliente.
type ParaBorde interface {
	Intentar(ctx context.Context, p PeticionDeIntento) (Resultado, error)
	HacerRollback(ctx context.Context, p PeticionDeRollback) (Resultado, error)
}
