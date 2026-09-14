package pipeline

import (
	"context"
	"errors"
	"fmt"

	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
)

// Pipelinecode es a QUÉ se le está pasando la validación: dónde está el
// pipelinecode clonado y para qué ambiente se va a ejecutar.
//
// Es un struct y no dos parámetros sueltos porque la firma ya creció una vez
// —la spec 13 le añadió la ruta, porque una regla suya habla de un ARCHIVO del
// step y no sólo del nombre de su directorio— y vuelve a crecer aquí: las reglas
// de la spec 14 leen `variables/<ambiente>/<paso>.yaml`, que sólo se sabe cuál es
// con el ambiente en la mano. La apuesta salió bien a la primera: la spec 15
// añade la gramática de `rules` sin tocar ni la firma ni el conjunto de reglas,
// porque `StepConfigRule` valida traduciendo y no enumerando campos.
//
// El ambiente es el EN EJECUCIÓN, no todos los del pipelinecode: un `prod` mal
// escrito no debe impedir un despliegue a `sand`, que es trabajo que sí se puede
// hacer bien.
type Pipelinecode struct {
	LocalPath   string
	Environment string
}

// PipelineStructureValidator comprueba el invariante estructural del
// pipelinecode: qué es un conjunto de steps ejecutable. Se invoca desde
// `04_steps_loader_handler`, que es el punto donde el formato en disco entra al
// modelo, y ANTES de que se ejecute el primer step: descubrir un typo a mitad
// del despliegue es el peor momento posible, porque los steps anteriores ya
// tuvieron efectos reales (spec 04 §4, opción C).
type PipelineStructureValidator interface {
	Validate(ctx *context.Context, code Pipelinecode, entries []StepEntry) error
}

// StepStructureRule es una regla del invariante. Es una Specification y no un
// eslabón de una cadena de responsabilidad a propósito: la validación no corta
// en el primer fallo, reporta todos los problemas estructurales de una vez
// (spec 04 §5.3').
//
// Su punto de extensión se conserva entero desde la spec 04: recibe directorios,
// su veredicto es un error y sólo sabe abortar. Lo que cambió dos veces es el
// contexto que se le da —la ruta con la spec 13, el ambiente con la 14—, y desde
// esta va en un `Pipelinecode` para que crecer deje de ser un cambio de firma.
type StepStructureRule interface {
	Name() string
	IsSatisfiedBy(ctx *context.Context, code Pipelinecode, entries []StepEntry) error
}

// StepsStructureValidator compone las reglas. Las specs 05, 13 y 14 añadieron
// reglas aquí sin tocar el repositorio ni el handler; la 15 no añadió ninguna, y
// eso es un resultado y no una omisión — su gramática entra por `StepConfigRule`,
// que ya estaba.
type StepsStructureValidator struct {
	rules []StepStructureRule
}

var _ PipelineStructureValidator = (*StepsStructureValidator)(nil)

// NewPipelineStructureValidator devuelve el validador con las reglas vigentes.
func NewPipelineStructureValidator(
	configs domStep.StepConfigRepository,
	manifests ManifestRepository,
	declarations domStep.VarsPipelineRepository,
	commands domStep.PipelineCommandRepository) PipelineStructureValidator {

	return NewStepsStructureValidator(
		NewStepEntryFormatRule(),
		NewUniqueStepOrderRule(),
		NewStepConfigRule(configs),
		NewDeclaredSourceVersionRule(manifests, declarations),
		NewVariableGraphRule(declarations, commands),
	)
}

func NewStepsStructureValidator(rules ...StepStructureRule) *StepsStructureValidator {
	return &StepsStructureValidator{rules: rules}
}

func (v *StepsStructureValidator) Validate(
	ctx *context.Context, code Pipelinecode, entries []StepEntry) error {

	errs := make([]error, 0, len(v.rules))
	for _, rule := range v.rules {
		if err := rule.IsSatisfiedBy(ctx, code, entries); err != nil {
			errs = append(errs, fmt.Errorf("[%s] %w", rule.Name(), err))
		}
	}
	return errors.Join(errs...)
}
