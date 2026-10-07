package dominio

import "context"

// Pipelines es el pipeline que se consulta, ya comprobado: de hoy si el commit está vacío.
type Pipelines interface {
	Pipeline(ctx context.Context, fuente Fuente, commit Commit) (Pipeline, error)
}

// Reservas dice qué ambientes se reservó el dueño del negocio.
type Reservas interface {
	// Reservado dice si el ambiente, por su valor, está reservado ahora mismo. Si nunca se reservó, no lo está.
	Reservado(ctx context.Context, ambiente string) (bool, error)
}
