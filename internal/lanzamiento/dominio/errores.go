package dominio

import (
	"errors"
	"fmt"
)

// ErrInvalido: lo que se pide construir no es válido, como un ambiente vacío o una versión menor que uno.
var ErrInvalido = errors.New("lanzamiento: inválido")

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrInvalido}, args...)...)
}
