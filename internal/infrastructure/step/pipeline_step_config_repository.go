package step

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	"gopkg.in/yaml.v3"
)

// StepConfigFileName es el archivo donde un step declara lo que es. Se nombra
// aquí, y no en el dominio, porque es un detalle del formato en disco.
const StepConfigFileName = "config.yaml"

type pipelineStepConfigRepository struct{}

var _ domStep.StepConfigRepository = (*pipelineStepConfigRepository)(nil)

func NewPipelineStepConfigRepository() domStep.StepConfigRepository {
	return &pipelineStepConfigRepository{}
}

// Get distingue TRES situaciones, y las tres son distintas a propósito
// (spec 13 §5.3 y §7):
//
//   - el archivo no está        ⇒ `NoStepConfig()`, sin error. El step no
//     declara ámbito: se ejecutará siempre y no persistirá registro.
//   - el archivo está y declara ⇒ la configuración, con su ámbito y sus reglas.
//   - el archivo está y NO declara un ámbito del vocabulario cerrado, o declara
//     una regla que el motor no conoce ⇒ ERROR, nombrando el directorio. Un
//     `config.yaml` presente es una intención de declarar, así que tratarlo como
//     ausente sería adivinar cuál.
//
// **La gramática se valida AL TRADUCIR, y eso es lo que la lleva gratis al
// validador de la spec 04**: `StepScopeRule` llama a este mismo `Get` para cada
// step antes del primero, así que una regla desconocida o un
// `state_changed: [project]` abortan la ejecución sin añadir una regla nueva al
// validador. Lo que no cabe ahí es la ADVERTENCIA de `max_age` sin
// `state_changed` —el puerto sólo sabe abortar—, y por eso vive en el handler
// que carga la configuración (spec 15 §5.4).
//
// El error se envuelve con la ruta relativa del archivo —no con la absoluta—
// porque quien lo lee edita el pipelinecode, no el directorio de trabajo que el
// motor clonó.
func (r *pipelineStepConfigRepository) Get(
	_ *context.Context, pipelineLocalPath, step string) (domStep.StepConfig, error) {

	relPath := filepath.ToSlash(filepath.Join("steps", step, StepConfigFileName))
	data, err := os.ReadFile(filepath.Join(pipelineLocalPath, "steps", step, StepConfigFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return domStep.NoStepConfig(), nil
		}
		return domStep.StepConfig{}, fmt.Errorf("leer '%s': %w", relPath, err)
	}

	var dto PipelineStepConfigDTO
	if err := yaml.Unmarshal(data, &dto); err != nil {
		return domStep.StepConfig{}, fmt.Errorf("parsear YAML de '%s': %w", relPath, err)
	}

	// Un archivo vacío entra por aquí con `Scope` vacío y sale como error, no
	// como ausencia: está presente, luego alguien quiso declarar algo.
	scope, err := domStep.NewScope(dto.Scope)
	if err != nil {
		return domStep.StepConfig{}, fmt.Errorf("'%s' %w", relPath, err)
	}

	rules, err := rulesOf(dto.Rules)
	if err != nil {
		return domStep.StepConfig{}, fmt.Errorf("'%s' %w", relPath, err)
	}
	return domStep.NewStepConfig(scope, rules)
}

// rulesOf traduce la lista declarada al conjunto del dominio, conservando el
// ORDEN: es el orden en que se evalúa el OR y por tanto el que decide qué motivo
// se emite cuando más de una regla se cumple.
func rulesOf(declared []PipelineStepRuleDTO) (domStep.RuleSet, error) {
	if len(declared) == 0 {
		return domStep.EmptyRuleSet(), nil
	}

	rules := make([]domStep.Rule, 0, len(declared))
	for _, dto := range declared {
		rule, err := ruleOf(dto)
		if err != nil {
			return domStep.RuleSet{}, err
		}
		rules = append(rules, rule)
	}
	return domStep.NewRuleSet(rules...)
}

// ruleOf es el único punto donde una clave escrita en el archivo se convierte en
// una regla del dominio. El vocabulario es CERRADO: una regla que el motor no
// conoce es un error de pipelinecode, no una regla que se ignora — ignorarla
// dejaría a quien la escribió creyendo que su step tiene una comprobación que no
// tiene.
func ruleOf(dto PipelineStepRuleDTO) (domStep.Rule, error) {
	switch domStep.RuleKind(dto.Name) {
	case domStep.RuleKindStateChanged:
		if dto.Shorthand {
			return domStep.NewDefaultStateChangedRule(), nil
		}
		var sources []string
		if err := dto.Value.Decode(&sources); err != nil {
			return nil, fmt.Errorf(
				"'%s' toma la lista de fuentes que vigila ('[%s]' o '[%s, %s]')",
				domStep.RuleKindStateChanged,
				domStep.StateSourcePipeline,
				domStep.StateSourcePipeline, domStep.StateSourceProject)
		}
		return domStep.NewStateChangedRule(sources)

	case domStep.RuleKindMaxAge:
		if dto.Shorthand {
			return nil, fmt.Errorf(
				"'%s' necesita una duración: '%s: 24h'",
				domStep.RuleKindMaxAge, domStep.RuleKindMaxAge)
		}
		var duration string
		if err := dto.Value.Decode(&duration); err != nil {
			return nil, fmt.Errorf(
				"'%s' toma una duración entre comillas o sin ellas ('30m', '6h', '720h')",
				domStep.RuleKindMaxAge)
		}
		return domStep.NewMaxAgeRule(duration)

	default:
		return nil, fmt.Errorf(
			"'%s' no es una regla de re-ejecución: se espera '%s' o '%s'",
			dto.Name, domStep.RuleKindStateChanged, domStep.RuleKindMaxAge)
	}
}
