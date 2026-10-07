package publicado

import "errors"

var (
	// ErrInvalido: lo pedido no es válido, como una fuente vacía o una fuente o un commit que no están donde se dice.
	ErrInvalido = errors.New("catalogo: inválido")

	// ErrRechazado: la petición era válida, pero el pipeline no pasa la comprobación y no se puede leer.
	ErrRechazado = errors.New("catalogo: rechazado")
)
