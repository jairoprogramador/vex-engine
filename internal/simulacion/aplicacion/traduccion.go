package aplicacion

import (
	"errors"

	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// traducir lleva los errores del dominio al lenguaje publicado, sin perder su mensaje ni el error concreto que
// llevan envuelto.
func traducir(err error) error {
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

// traducirFallos lleva los fallos de comprobación de Definición al lenguaje publicado de Simulación — un
// mapeo campo a campo, sin capa de dominio intermedia: Fallo no protege ninguna invariante propia de
// Simulación, solo reporta.
func traducirFallos(fallos []definicionpublicado.Fallo) []publicado.Fallo {
	traducidos := make([]publicado.Fallo, len(fallos))
	for i, f := range fallos {
		traducidos[i] = publicado.Fallo{
			Invariante: f.Invariante, Fichero: f.Fichero, Paso: f.Paso, Ambiente: f.Ambiente, Detalle: f.Detalle,
		}
	}
	return traducidos
}
