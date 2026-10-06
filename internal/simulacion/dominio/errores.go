package dominio

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalido: lo que se pide construir o simular no es válido, como un ámbito de ambiente sin nombre.
	ErrInvalido = errors.New("simulacion: inválido")
)

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrInvalido}, args...)...)
}
