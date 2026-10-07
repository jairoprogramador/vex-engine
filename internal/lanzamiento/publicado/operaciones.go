package publicado

import "context"

// ParaBorde es lo que Lanzamiento ofrece al borde del motor (RD-10 lo conecta).
type ParaBorde interface {
	// Lanzar hace visible un despliegue a su destinatario. Es incondicional: la reserva de un ambiente solo
	// bloquea el lanzamiento en nombre del actor ausente, nunca al dueño del negocio. Sin nombre, el nombre
	// toma el valor de la versión.
	Lanzar(ctx context.Context, ambiente, despliegue, nombre string) (Lanzamiento, error)
	// Reservar: desde ahora, el dueño del negocio decide él los lanzamientos de ese ambiente.
	Reservar(ctx context.Context, ambiente string) error
	// Liberar: desde ahora, se vuelve a lanzar en su nombre en cuanto un despliegue quede listo.
	Liberar(ctx context.Context, ambiente string) error
	// LanzamientosDeUnAmbiente son los de ese ambiente, en el orden en que se lanzaron. Sin lanzamientos, una
	// lista vacía.
	LanzamientosDeUnAmbiente(ctx context.Context, ambiente string) ([]Lanzamiento, error)
}
