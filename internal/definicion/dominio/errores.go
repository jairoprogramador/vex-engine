package dominio

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalido     = errors.New("definicion: petición inválida")
	ErrNoExiste     = errors.New("definicion: no existe")
	ErrNoComprobado = errors.New("definicion: el pipeline no pasa la comprobación")
)

type Invariante string

const (
	Formato           Invariante = "formato"
	Pasos             Invariante = "pasos"
	Variables         Invariante = "variables"
	VariablesDeSalida Invariante = "variables de salida"
	Aserciones        Invariante = "aserciones"
	Ambientes         Invariante = "ambientes"
)

type Fallo struct {
	Invariante Invariante
	Fichero    string
	Paso       string
	Ambiente   string
	Detalle    string
}

func (f Fallo) String() string {
	var b strings.Builder
	if f.Fichero != "" {
		b.WriteString(f.Fichero)
		b.WriteString(": ")
	}
	if f.Ambiente != "" {
		fmt.Fprintf(&b, "en el ambiente %s, ", f.Ambiente)
	}
	fmt.Fprintf(&b, "%s (%s)", f.Detalle, f.Invariante)
	return b.String()
}

type FallosDeComprobacion struct {
	Fallos []Fallo
}

func (e *FallosDeComprobacion) Error() string {
	lineas := make([]string, 0, len(e.Fallos)+1)
	lineas = append(lineas, ErrNoComprobado.Error()+":")
	for _, f := range e.Fallos {
		lineas = append(lineas, "  - "+f.String())
	}
	return strings.Join(lineas, "\n")
}

func (e *FallosDeComprobacion) Is(objetivo error) bool { return objetivo == ErrNoComprobado }
