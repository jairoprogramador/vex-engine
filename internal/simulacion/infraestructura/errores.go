package infraestructura

import (
	"errors"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

// errorDeArriba es el error de un contexto de arriba visto desde el dominio de Simulación: sigue diciendo lo mismo
// y sigue siendo el error concreto que lo causó (errors.Is, errors.As), pero además es un error de este dominio,
// que es el que Simulación sabe llevar al lenguaje publicado.
type errorDeArriba struct {
	dominio error
	causa   error
}

func (e *errorDeArriba) Error() string   { return e.causa.Error() }
func (e *errorDeArriba) Unwrap() []error { return []error{e.dominio, e.causa} }

// delContextoDeArriba traduce, como un ACL, lo que Definición publica de un pipeline que no está donde se dice
// o que no se puede pedir: es un ErrInvalido, que quien invoca puede corregir. Lo demás pasa igual. En
// particular, un pipeline que no pasa la comprobación no es un error de esta petición sino su resultado
// (SIM-1 §1): la aplicación lo reconoce por *definicionpublicado.FallosDeComprobacion y no debe quedar
// convertido en otra cosa.
func delContextoDeArriba(err error) error {
	if errors.Is(err, definicionpublicado.ErrNoExiste) || errors.Is(err, definicionpublicado.ErrInvalido) {
		return &errorDeArriba{dominio: dominio.ErrInvalido, causa: err}
	}
	return err
}
