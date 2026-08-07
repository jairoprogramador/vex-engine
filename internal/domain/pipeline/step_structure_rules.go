package pipeline

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

const (
	StepEntryFormatRuleName = "formato_del_directorio"
	UniqueStepOrderRuleName = "orden_único"
	StepScopeRuleName       = "ámbito_declarado"
)

// StepEntryFormatRule exige que todo directorio bajo `steps/` case
// `^\d{2}-.+$`. Nombra en el error cada directorio culpable: hasta la spec 04
// un `4_test` desaparecía del pipeline sin un mensaje y un `2-supply` pasaba sin
// ejecutar nada (spec 04 §1 b′, c).
type StepEntryFormatRule struct{}

var _ StepStructureRule = (*StepEntryFormatRule)(nil)

func NewStepEntryFormatRule() StepEntryFormatRule {
	return StepEntryFormatRule{}
}

func (r StepEntryFormatRule) Name() string { return StepEntryFormatRuleName }

func (r StepEntryFormatRule) IsSatisfiedBy(_ *context.Context, _ string, entries []StepEntry) error {
	errs := make([]error, 0, len(entries))
	for _, entry := range entries {
		if _, err := command.NewStepName(entry.String()); err != nil {
			errs = append(errs, fmt.Errorf(
				"el directorio 'steps/%s' no es un step válido: se espera '%s'",
				entry, command.StepDirNameFormat))
		}
	}
	return errors.Join(errs...)
}

// UniqueStepOrderRule exige que dos steps no declaren el mismo orden: con
// `02-a` y `02-b` el orden de ejecución sería arbitrario y las dos claves de
// estado colisionarían.
//
// Los directorios que no cumplen el formato se ignoran aquí: de esos ya habla
// StepEntryFormatRule, y repetirlos sumaría ruido al mismo diagnóstico.
type UniqueStepOrderRule struct{}

var _ StepStructureRule = (*UniqueStepOrderRule)(nil)

func NewUniqueStepOrderRule() UniqueStepOrderRule {
	return UniqueStepOrderRule{}
}

func (r UniqueStepOrderRule) Name() string { return UniqueStepOrderRuleName }

func (r UniqueStepOrderRule) IsSatisfiedBy(_ *context.Context, _ string, entries []StepEntry) error {
	byOrder := make(map[int][]string, len(entries))
	for _, entry := range entries {
		stepName, err := command.NewStepName(entry.String())
		if err != nil {
			continue
		}
		byOrder[stepName.Order()] = append(byOrder[stepName.Order()], entry.String())
	}

	duplicated := make([]int, 0)
	for order, names := range byOrder {
		if len(names) > 1 {
			duplicated = append(duplicated, order)
		}
	}
	slices.Sort(duplicated)

	errs := make([]error, 0, len(duplicated))
	for _, order := range duplicated {
		names := byOrder[order]
		slices.Sort(names)
		errs = append(errs, fmt.Errorf(
			"el orden %02d lo declaran varios directorios: %s",
			order, strings.Join(quoted(names), ", ")))
	}
	return errors.Join(errs...)
}

// StepScopeRule exige que todo `config.yaml` PRESENTE declare un `scope` del
// vocabulario cerrado (spec 13 §5.1).
//
// Lo que NO exige es que el archivo exista: un step sin `config.yaml` es
// legítimo —se ejecuta siempre y no persiste registro (§5.3)— y por eso el
// repositorio devuelve ausencia sin error. La regla sólo se pronuncia sobre lo
// que alguien escribió.
//
// Corre aquí y no en la cadena de step porque un ámbito mal escrito descubierto
// a mitad del despliegue llega tarde: los steps anteriores ya crearon recursos
// reales. Es la misma razón por la que existe el validador (spec 04 §4).
type StepScopeRule struct {
	configs domStep.StepConfigRepository
}

var _ StepStructureRule = (*StepScopeRule)(nil)

func NewStepScopeRule(configs domStep.StepConfigRepository) StepScopeRule {
	return StepScopeRule{configs: configs}
}

func (r StepScopeRule) Name() string { return StepScopeRuleName }

func (r StepScopeRule) IsSatisfiedBy(
	ctx *context.Context, pipelineLocalPath string, entries []StepEntry) error {

	errs := make([]error, 0, len(entries))
	for _, entry := range entries {
		// El error del repositorio ya nombra el archivo —y con él el directorio—,
		// así que no se le añade contexto: duplicarlo haría el mensaje más largo
		// sin decir nada nuevo.
		if _, err := r.configs.Get(ctx, pipelineLocalPath, entry.String()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func quoted(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, fmt.Sprintf("'steps/%s'", name))
	}
	return out
}
