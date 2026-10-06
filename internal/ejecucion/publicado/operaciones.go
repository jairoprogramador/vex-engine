package publicado

import "context"

// Lo que Ejecución de Pipeline publica: intentar hasta un paso en un ambiente, y hacer rollback a un destino
// (docs/modelo/contextos/ejecucion.md, «Lo que publica»). La salida de los comandos no sale por aquí: se
// guarda en el Historial y se consulta de allí.

// ParaBorde es lo que usa el borde mínimo (RD-06) — su único cliente.
//
// entorno son las variables de entorno que quien invoca pide para los comandos del intento. Viaja al lado de la
// petición, no dentro: no es parte de lo que el Historial guarda ni de lo que se compara entre intentos. Sin
// variables, nil.
type ParaBorde interface {
	Intentar(ctx context.Context, p PeticionDeIntento, entorno Entorno) (Resultado, error)
	HacerRollback(ctx context.Context, p PeticionDeRollback, entorno Entorno) (Resultado, error)
}
