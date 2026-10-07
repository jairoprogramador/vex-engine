package aplicacion

import (
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/catalogo/dominio"
	"github.com/jairoprogramador/vex-engine/internal/catalogo/publicado"
)

// traducir lleva los errores del dominio a los del lenguaje publicado, sin perder su mensaje.
func traducir(err error) error {
	var destino error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, dominio.ErrInvalido):
		destino = publicado.ErrInvalido
	case errors.Is(err, dominio.ErrRechazado):
		destino = publicado.ErrRechazado
	default:
		return err
	}
	return &errorTraducido{publicado: destino, causa: err}
}

type errorTraducido struct {
	publicado error
	causa     error
}

func (e *errorTraducido) Error() string   { return e.causa.Error() }
func (e *errorTraducido) Unwrap() []error { return []error{e.publicado, e.causa} }
