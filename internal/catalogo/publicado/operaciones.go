package publicado

import "context"

// ParaBorde es lo que Catálogo ofrece al borde del motor. Son consultas: no escriben nada.
type ParaBorde interface {
	// Ambientes son los del pipeline, en su orden, cada uno con su estado de reserva. Un ambiente que nunca se
	// reservó no está reservado.
	Ambientes(ctx context.Context, p PeticionDeCatalogo) ([]Ambiente, error)
	// Pasos son los del pipeline, en su orden.
	Pasos(ctx context.Context, p PeticionDeCatalogo) ([]Paso, error)
}
