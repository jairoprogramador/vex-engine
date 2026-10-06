package publicado

import "errors"

var (
	// ErrInvalido: lo que se pide no se puede pedir, como una petición sin fuente ni copia de trabajo, o con
	// las dos a la vez.
	ErrInvalido = errors.New("simulacion: petición inválida")
)
