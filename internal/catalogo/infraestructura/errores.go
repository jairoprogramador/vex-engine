package infraestructura

import (
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/dominio"
	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
)

// errorDeArriba es el error de un contexto de arriba visto desde el dominio de Catálogo: sigue siendo el error
// concreto que lo causó (errors.Is, errors.As), pero además es un error de este dominio, que es el que Catálogo
// sabe llevar al lenguaje publicado.
type errorDeArriba struct {
	dominio error
	causa   error
}

func (e *errorDeArriba) Error() string   { return e.causa.Error() }
func (e *errorDeArriba) Unwrap() []error { return []error{e.dominio, e.causa} }

// delContextoDeArriba traduce, como un ACL, los errores que publica Definición (que ya traduce los de Suministro):
//   - lo que se apunta y no está (la fuente, el commit) o no se puede pedir es un ErrInvalido: quien invoca lo
//     puede corregir, igual que en Ejecución;
//   - un pipeline que no pasa la comprobación es un ErrRechazado.
//
// Lo que no reconoce lo devuelve igual: es un fallo de verdad.
func delContextoDeArriba(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, definicionpublicado.ErrNoExiste), errors.Is(err, definicionpublicado.ErrInvalido):
		return &errorDeArriba{dominio: dominio.ErrInvalido, causa: err}
	case errors.Is(err, definicionpublicado.ErrNoComprobado):
		return &errorDeArriba{dominio: dominio.ErrRechazado, causa: err}
	}
	return err
}
