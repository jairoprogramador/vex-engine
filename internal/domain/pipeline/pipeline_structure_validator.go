package pipeline

import (
	"context"
	"errors"
	"fmt"

	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

// PipelineStructureValidator comprueba el invariante estructural del
// pipelinecode: qué es un conjunto de steps ejecutable. Se invoca desde
// `04_steps_loader_handler`, que es el punto donde el formato en disco entra al
// modelo, y ANTES de que se ejecute el primer step: descubrir un typo a mitad
// del despliegue es el peor momento posible, porque los steps anteriores ya
// tuvieron efectos reales (spec 04 §4, opción C).
type PipelineStructureValidator interface {
	Validate(ctx *context.Context, pipelineLocalPath string, entries []StepEntry) error
}

// StepStructureRule es una regla del invariante. Es una Specification y no un
// eslabón de una cadena de responsabilidad a propósito: la validación no corta
// en el primer fallo, reporta todos los problemas estructurales de una vez
// (spec 04 §5.3').
//
// La firma ganó la ruta del pipelinecode con la spec 13, y es la única
// desviación de lo que aquella spec previó (§5.2'): una regla que «habla de un
// archivo del step» no puede leerlo con el nombre del directorio a secas. Lo que
// sí se conserva entero es su punto de extensión —recibe directorios, su
// veredicto es un error y sólo sabe abortar—, así que la regla nueva entra sin
// tocar las dos que ya estaban, que ignoran los dos parámetros nuevos.
type StepStructureRule interface {
	Name() string
	IsSatisfiedBy(ctx *context.Context, pipelineLocalPath string, entries []StepEntry) error
}

// StepsStructureValidator compone las reglas. Las specs 05, 13 y 15 añaden
// reglas aquí sin tocar el repositorio ni el handler.
type StepsStructureValidator struct {
	rules []StepStructureRule
}

var _ PipelineStructureValidator = (*StepsStructureValidator)(nil)

// NewPipelineStructureValidator devuelve el validador con las reglas vigentes.
func NewPipelineStructureValidator(configs domStep.StepConfigRepository) PipelineStructureValidator {
	return NewStepsStructureValidator(
		NewStepEntryFormatRule(),
		NewUniqueStepOrderRule(),
		NewStepScopeRule(configs),
	)
}

func NewStepsStructureValidator(rules ...StepStructureRule) *StepsStructureValidator {
	return &StepsStructureValidator{rules: rules}
}

func (v *StepsStructureValidator) Validate(
	ctx *context.Context, pipelineLocalPath string, entries []StepEntry) error {

	errs := make([]error, 0, len(v.rules))
	for _, rule := range v.rules {
		if err := rule.IsSatisfiedBy(ctx, pipelineLocalPath, entries); err != nil {
			errs = append(errs, fmt.Errorf("[%s] %w", rule.Name(), err))
		}
	}
	return errors.Join(errs...)
}
