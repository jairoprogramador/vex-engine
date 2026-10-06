package publicado

import (
	"errors"
	"strings"
)

var (
	// ErrInvalido: lo que se pide no se puede pedir, como un commit que no es un identificador completo.
	ErrInvalido = errors.New("definicion: petición inválida")

	// ErrNoExiste: la fuente del pipeline o su commit no están donde se dice.
	ErrNoExiste = errors.New("definicion: no existe")

	// ErrNoComprobado: lo escrito no pasa la comprobación. Lo que falla está en *FallosDeComprobacion.
	ErrNoComprobado = errors.New("definicion: el pipeline no pasa la comprobación")
)

type Fallo struct {
	Invariante string
	Fichero    string
	Paso       string
	Ambiente   string
	Detalle    string
}

type FallosDeComprobacion struct {
	Fallos  []Fallo
	mensaje string
}

func NuevosFallosDeComprobacion(fallos []Fallo, mensaje string) *FallosDeComprobacion {
	return &FallosDeComprobacion{Fallos: fallos, mensaje: mensaje}
}

func (e *FallosDeComprobacion) Error() string {
	if e.mensaje != "" {
		return e.mensaje
	}
	detalles := make([]string, len(e.Fallos))
	for i, f := range e.Fallos {
		detalles[i] = f.Fichero + ": " + f.Detalle
	}
	return ErrNoComprobado.Error() + ": " + strings.Join(detalles, "; ")
}

func (e *FallosDeComprobacion) Is(objetivo error) bool { return objetivo == ErrNoComprobado }
