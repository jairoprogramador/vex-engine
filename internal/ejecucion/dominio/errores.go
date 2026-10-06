package dominio

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalido: lo que se pide construir no es válido, como un nombre vacío o un paso que no está en la
	// lista.
	ErrInvalido = errors.New("ejecucion: inválido")

	// ErrRechazado: la operación rompería una invariante del agregado, y no se aplica.
	ErrRechazado = errors.New("ejecucion: rechazado")

	// ErrNoDisponible: el espacio de trabajo del ambiente no se puede alcanzar. El intento no empieza (EJ-5,
	// DEC-06.18).
	ErrNoDisponible = errors.New("ejecucion: espacio de trabajo no disponible")
)

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrInvalido}, args...)...)
}

func rechazo(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrRechazado}, args...)...)
}
