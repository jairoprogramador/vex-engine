package dominio

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalido: lo que se pide construir no es válido, como un nombre vacío o un origen que no existe.
	ErrInvalido = errors.New("resolucion: inválido")

	// ErrRechazado: la operación rompería una invariante del agregado, y no se aplica.
	ErrRechazado = errors.New("resolucion: rechazado")
)

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrInvalido}, args...)...)
}

func rechazo(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrRechazado}, args...)...)
}
