package aplicacion

import (
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
)

func lanzamientoAPublicado(l dominio.LanzamientoRegistrado) publicado.Lanzamiento {
	return publicado.Lanzamiento{
		Ambiente: l.Ambiente.String(), Despliegue: l.Despliegue.String(),
		Version: l.Version.Numero(), Nombre: l.Nombre.String(), Instante: l.Instante,
	}
}

// traducir lleva los errores del dominio a los del lenguaje publicado, sin perder su mensaje.
func traducir(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, dominio.ErrInvalido) {
		return &errorTraducido{publicado: publicado.ErrInvalido, causa: err}
	}
	return err
}

type errorTraducido struct {
	publicado error
	causa     error
}

func (e *errorTraducido) Error() string   { return e.causa.Error() }
func (e *errorTraducido) Unwrap() []error { return []error{e.publicado, e.causa} }
