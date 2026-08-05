package pipeline

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

const (
	StepEntryFormatRuleName = "formato_del_directorio"
	UniqueStepOrderRuleName = "orden_único"
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

func (r StepEntryFormatRule) IsSatisfiedBy(entries []StepEntry) error {
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

func (r UniqueStepOrderRule) IsSatisfiedBy(entries []StepEntry) error {
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

func quoted(names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, fmt.Sprintf("'steps/%s'", name))
	}
	return out
}
