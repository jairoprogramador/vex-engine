package dominio

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalido: lo que se pide no es válido, como una fuente vacía o una fuente o un commit que no están donde
	// se dice.
	ErrInvalido = errors.New("catalogo: inválido")
	// ErrRechazado: el pipeline no pasa la comprobación, así que no se puede dar su catálogo.
	ErrRechazado = errors.New("catalogo: rechazado")
)

// ParametroInvalidoError es un ErrInvalido que dice qué campo y qué valor no valen, para que quien presenta el
// error no tenga que leer el texto.
type ParametroInvalidoError struct {
	Campo  string
	Valor  string
	Motivo string
}

func (e *ParametroInvalidoError) Error() string { return ErrInvalido.Error() + ": " + e.Motivo }

func (e *ParametroInvalidoError) Is(destino error) bool { return destino == ErrInvalido }

func (e *ParametroInvalidoError) ParametroInvalido() (campo, valor string) { return e.Campo, e.Valor }

func invalido(campo, valor, formato string, args ...any) error {
	return &ParametroInvalidoError{Campo: campo, Valor: valor, Motivo: fmt.Sprintf(formato, args...)}
}
