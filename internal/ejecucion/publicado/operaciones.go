package publicado

import "context"

// Lo que Ejecución de Pipeline publica: intentar hasta un paso en un ambiente, y hacer rollback a un destino
// (docs/modelo/contextos/ejecucion.md, «Lo que publica»). Las dos reciben a dónde entregar la salida de los
// comandos (DEC-12.5).

// ParaBorde es lo que usa el borde mínimo (RD-06) — su único cliente.
type ParaBorde interface {
	Intentar(ctx context.Context, p PeticionDeIntento, salida Salida) (Resultado, error)
	HacerRollback(ctx context.Context, p PeticionDeRollback, salida Salida) (Resultado, error)
}

// Salida entrega, en vivo y tal cual, lo que un comando de un paso imprime. No se guarda (DEC-12.5): no es un
// registro ni una consulta, así que la promesa de no exponer valores no la alcanza (DEC-08.8).
type Salida interface {
	Escribir(paso string, datos []byte) error
}
