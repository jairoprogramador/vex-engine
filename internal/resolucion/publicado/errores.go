package publicado

import (
	"errors"
	"fmt"
)

// ErrRechazado: la operación pedida rompería una invariante del dominio, y no se aplica.
var ErrRechazado = errors.New("resolucion: rechazado")

// VariableNoEncontradaError: un texto a interpolar usa un nombre que no está disponible.
type VariableNoEncontradaError struct{ Nombre string }

func (e *VariableNoEncontradaError) Error() string {
	return fmt.Sprintf("%v: la variable %q no está disponible para interpolar", ErrRechazado, e.Nombre)
}

func (e *VariableNoEncontradaError) Is(objetivo error) bool { return objetivo == ErrRechazado }
