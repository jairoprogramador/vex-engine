package pipeline

import (
	"fmt"
	"slices"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

// StepEntry es el nombre CRUDO de un directorio bajo `steps/`, tal como está en
// disco. Es deliberadamente una cadena y no un command.StepName ya construido:
// la validación tiene que poder hablar de lo que NO se pudo convertir en step, y
// un StepName no puede representarlo (spec 04 §5.1').
type StepEntry string

func (e StepEntry) String() string {
	return string(e)
}

// NewStepNames convierte las entradas —ya validadas— en StepName y las devuelve
// en ORDEN NUMÉRICO por el prefijo.
//
// El orden se enuncia aquí en vez de heredarse del que devuelve `os.ReadDir`.
// Con dos dígitos ambos coinciden, porque `%02d` es de ancho fijo y el orden
// lexicográfico de cadenas de ancho fijo sobre dígitos ES el numérico; ordenar
// en el dominio hace que el contrato se lea en una línea y que siga siendo
// correcto aunque una de esas dos premisas cambie (spec 04 §5.2).
func NewStepNames(entries []StepEntry) ([]command.StepName, error) {
	stepNames := make([]command.StepName, 0, len(entries))
	for _, entry := range entries {
		stepName, err := command.NewStepName(entry.String())
		if err != nil {
			return nil, fmt.Errorf("directorio de step '%s': %w", entry, err)
		}
		stepNames = append(stepNames, stepName)
	}

	slices.SortFunc(stepNames, func(a, b command.StepName) int {
		return a.Order() - b.Order()
	})
	return stepNames, nil
}
