package aplicacion

import (
	definicionpublicado "github.com/jairoprogramador/vex-engine/internal/definicion/publicado"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

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
