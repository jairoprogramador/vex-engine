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
	// campo y valor son el campo de la petición al que apunta el rechazo, si el adaptador lo sabe sin adivinar.
	campo, valor string
}

func (e *errorDeArriba) Error() string   { return e.causa.Error() }
func (e *errorDeArriba) Unwrap() []error { return []error{e.dominio, e.causa} }

func (e *errorDeArriba) ParametroInvalido() (campo, valor string) { return e.campo, e.valor }

// delContextoDeArriba traduce, como un ACL, lo que Definición publica de un pipeline que no está donde se dice
// o que no se puede pedir: es un ErrInvalido, que quien invoca puede corregir. Lo demás pasa igual. En
// particular, un pipeline que no pasa la comprobación no es un error de esta petición sino su resultado
// (SIM-1 §1): la aplicación lo reconoce por *definicionpublicado.FallosDeComprobacion y no debe quedar
// convertido en otra cosa.
func delContextoDeArriba(err error) error { return delContextoDeArribaEnElCampo(err, "", "") }

// delContextoDeArribaEnElCampo es delContextoDeArriba para lo que se pidió con un campo de la petición que el
// adaptador conoce: dice cuál campo y qué valor. Si no se sabe sin adivinar, se usa delContextoDeArriba.
func delContextoDeArribaEnElCampo(err error, campo, valor string) error {
	if errors.Is(err, definicionpublicado.ErrNoExiste) || errors.Is(err, definicionpublicado.ErrInvalido) {
		return &errorDeArriba{dominio: dominio.ErrInvalido, causa: err, campo: campo, valor: valor}
	}
	return err
}
