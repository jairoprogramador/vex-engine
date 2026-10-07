package infraestructura

import (
	"errors"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	resolucionpublicado "github.com/jairoprogramador/vex-engine/internal/resolucion/publicado"
	suministropublicado "github.com/jairoprogramador/vex-engine/internal/suministro/publicado"
)

// errorDeArriba es el error de un contexto de arriba visto desde el dominio de Ejecución: sigue diciendo lo mismo
// y sigue siendo el error concreto que lo causó (errors.Is, errors.As), pero además es un error de este dominio,
// que es el que Ejecución sabe llevar al lenguaje publicado.
type errorDeArriba struct {
	dominio error
	causa   error
	// campo y valor son el campo de la petición al que apunta el rechazo, si el adaptador lo sabe sin adivinar.
	campo, valor string
}

func (e *errorDeArriba) Error() string   { return e.causa.Error() }
func (e *errorDeArriba) Unwrap() []error { return []error{e.dominio, e.causa} }

func (e *errorDeArriba) ParametroInvalido() (campo, valor string) { return e.campo, e.valor }

// delContextoDeArriba traduce, como un ACL, los errores que publican Definición, Suministro y Resolución a los de
// este dominio, para que no se pierdan en «interno»:
//   - lo que se apunta y no está (la fuente, el commit, la copia de trabajo) o no se puede pedir es un
//     ErrInvalido: quien invoca lo puede corregir;
//   - un pipeline que no pasa la comprobación, o un texto que usa una variable que no existe, es un ErrRechazado:
//     la petición era válida, pero lo que el pipeline dice rompe una regla.
//
// Lo que no reconoce lo devuelve igual: es un fallo de verdad.
func delContextoDeArriba(err error) error { return delContextoDeArribaEnElCampo(err, "", "") }

// delContextoDeArribaEnElCampo es delContextoDeArriba para lo que se pidió con un campo de la petición que el
// adaptador conoce: si el rechazo es un ErrInvalido, dice cuál campo y qué valor. Si no se sabe sin adivinar, se
// usa delContextoDeArriba, que no dice ninguno.
func delContextoDeArribaEnElCampo(err error, campo, valor string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, definicionpublicado.ErrNoExiste), errors.Is(err, definicionpublicado.ErrInvalido),
		errors.Is(err, suministropublicado.ErrNoExiste), errors.Is(err, suministropublicado.ErrInvalido):
		return &errorDeArriba{dominio: dominio.ErrInvalido, causa: err, campo: campo, valor: valor}
	case errors.Is(err, definicionpublicado.ErrNoComprobado), errors.Is(err, resolucionpublicado.ErrRechazado):
		return &errorDeArriba{dominio: dominio.ErrRechazado, causa: err}
	}
	return err
}
