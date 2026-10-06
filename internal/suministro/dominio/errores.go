package dominio

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalido: lo que se pide no se puede pedir, como un commit que no es un identificador completo.
	ErrInvalido = errors.New("suministro: petición inválida")

	// ErrNoExiste: la fuente, la copia de trabajo o el commit no están donde se dice.
	ErrNoExiste = errors.New("suministro: no existe")
)

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrInvalido}, args...)...)
}
