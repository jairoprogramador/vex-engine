package dominio

import (
	"errors"
	"fmt"
)

// ErrInvalido: lo que se pide construir no es válido, como un ambiente o un hash vacíos.
var ErrInvalido = errors.New("diagnóstico: inválido")

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrInvalido}, args...)...)
}
