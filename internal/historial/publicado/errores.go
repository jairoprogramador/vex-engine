package publicado

import (
	"errors"
	"fmt"
)

var (
	// ErrRechazado: lo pedido rompería una regla del historial, y no se escribe nada.
	ErrRechazado = errors.New("historial: rechazado")

	// ErrNoExiste: lo que se pide no está en el historial.
	ErrNoExiste = errors.New("historial: no existe")

	// ErrEscrituraConcurrente: otros procesos escribieron en lo mismo, una y otra vez, y no se pudo escribir. No
	// se escribió nada; volver a pedirlo es seguro.
	ErrEscrituraConcurrente = errors.New("historial: otras escrituras ganaron en cada intento")
)

// AmbienteOcupadoError rechaza abrir un intento en un ambiente con otro en curso, y dice cuál. Ese intento
// se puede dar por abandonado.
type AmbienteOcupadoError struct {
	Ambiente string
	Intento  string
}

func (e *AmbienteOcupadoError) Error() string {
	return fmt.Sprintf("historial: el ambiente %q tiene en curso el intento %s", e.Ambiente, e.Intento)
}

func (e *AmbienteOcupadoError) Is(objetivo error) bool { return objetivo == ErrRechazado }
