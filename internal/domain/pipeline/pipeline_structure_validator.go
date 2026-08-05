package pipeline

import (
	"errors"
	"fmt"
)

// PipelineStructureValidator comprueba el invariante estructural del
// pipelinecode: qué es un conjunto de steps ejecutable. Se invoca desde
// `04_steps_loader_handler`, que es el punto donde el formato en disco entra al
// modelo, y ANTES de que se ejecute el primer step: descubrir un typo a mitad
// del despliegue es el peor momento posible, porque los steps anteriores ya
// tuvieron efectos reales (spec 04 §4, opción C).
type PipelineStructureValidator interface {
	Validate(entries []StepEntry) error
}

// StepStructureRule es una regla del invariante. Es una Specification y no un
// eslabón de una cadena de responsabilidad a propósito: la validación no corta
// en el primer fallo, reporta todos los problemas estructurales de una vez
// (spec 04 §5.3').
type StepStructureRule interface {
	Name() string
	IsSatisfiedBy(entries []StepEntry) error
}

// StepsStructureValidator compone las reglas. Las specs 05 y 15 añaden reglas
// aquí sin tocar el repositorio ni el handler.
type StepsStructureValidator struct {
	rules []StepStructureRule
}

var _ PipelineStructureValidator = (*StepsStructureValidator)(nil)

// NewPipelineStructureValidator devuelve el validador con las reglas vigentes.
func NewPipelineStructureValidator() PipelineStructureValidator {
	return NewStepsStructureValidator(
		NewStepEntryFormatRule(),
		NewUniqueStepOrderRule(),
	)
}

func NewStepsStructureValidator(rules ...StepStructureRule) *StepsStructureValidator {
	return &StepsStructureValidator{rules: rules}
}

func (v *StepsStructureValidator) Validate(entries []StepEntry) error {
	errs := make([]error, 0, len(v.rules))
	for _, rule := range v.rules {
		if err := rule.IsSatisfiedBy(entries); err != nil {
			errs = append(errs, fmt.Errorf("[%s] %w", rule.Name(), err))
		}
	}
	return errors.Join(errs...)
}
