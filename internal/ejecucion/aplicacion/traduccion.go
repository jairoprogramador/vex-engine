package aplicacion

import (
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// traducir lleva los errores del dominio al lenguaje publicado, sin perder su mensaje ni el error concreto que
// llevan envuelto — un *historialpublicado.AmbienteOcupadoError sigue distinguible con errors.As más arriba,
// aunque este paquete nunca importe historial/publicado (DEC-11.3).
func traducir(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, dominio.ErrInvalido):
		return &errorTraducido{publicado: publicado.ErrInvalido, causa: err}
	case errors.Is(err, dominio.ErrRechazado):
		return &errorTraducido{publicado: publicado.ErrRechazado, causa: err}
	case errors.Is(err, dominio.ErrNoDisponible):
		return &errorTraducido{publicado: publicado.ErrNoDisponible, causa: err}
	}
	return err
}

type errorTraducido struct {
	publicado error
	causa     error
}

func (e *errorTraducido) Error() string   { return e.causa.Error() }
func (e *errorTraducido) Unwrap() []error { return []error{e.publicado, e.causa} }

func resultadoAPublicado(intento string, desenlace dominio.Desenlace, despliegue string) publicado.Resultado {
	return publicado.Resultado{Intento: intento, Estado: desenlace.String(), Despliegue: despliegue}
}
