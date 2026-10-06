package dominio

import (
	"errors"
	"fmt"
)

var (
	// ErrRechazado: el registro rompería una invariante de su agregado, y no se escribe.
	ErrRechazado = errors.New("historial: registro rechazado")

	// ErrConflicto: alguien añadió un registro al agregado desde que se leyó. Se vuelve a leer y a decidir.
	ErrConflicto = errors.New("historial: el agregado cambió desde que se leyó")

	// ErrEscrituraConcurrente: se volvió a leer y a decidir muchas veces y otros escritores ganaron siempre. No
	// se escribió nada.
	ErrEscrituraConcurrente = errors.New("historial: otras escrituras ganaron en cada intento")

	// ErrNoExiste: lo que se pide no está en el historial.
	ErrNoExiste = errors.New("historial: no existe")
)

// AmbienteOcupadoError rechaza abrir un intento en un ambiente que ya tiene otro en curso, y dice cuál
// es (IT-07 DEC-07.8).
type AmbienteOcupadoError struct {
	Ambiente Ambiente
	Intento  IdIntento
}

func (e *AmbienteOcupadoError) Error() string {
	return fmt.Sprintf("%v: el ambiente %q tiene en curso el intento %s", ErrRechazado, e.Ambiente, e.Intento)
}

func (e *AmbienteOcupadoError) Is(objetivo error) bool { return objetivo == ErrRechazado }

func rechazo(formato string, args ...any) error {
	return fmt.Errorf("%w: "+formato, append([]any{ErrRechazado}, args...)...)
}
