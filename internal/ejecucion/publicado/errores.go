package publicado

import "errors"

var (
	// ErrInvalido: lo que se pide no se puede pedir, como un ambiente vacío o un paso que no está en el
	// pipeline.
	ErrInvalido = errors.New("ejecucion: petición inválida")

	// ErrRechazado: la operación rompería una invariante, como abrir un intento en un ambiente ya ocupado. El
	// error concreto sigue distinguible con errors.As (por ejemplo, *historialpublicado.AmbienteOcupadoError).
	ErrRechazado = errors.New("ejecucion: rechazado")

	// ErrNoDisponible: el espacio de trabajo del ambiente no se pudo alcanzar. El intento no empezó (EJ-5).
	ErrNoDisponible = errors.New("ejecucion: espacio de trabajo no disponible")
)
