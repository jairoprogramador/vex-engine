package rules

import (
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

// Rule es un alias de vos.Rule para uso en este paquete.
type Rule = vos.Rule

// allRules es la implementación composite — todas las reglas deben satisfacerse.
type allRules struct {
	rs []Rule
}

// AllRules devuelve una Rule compuesta que exige que todas las reglas dadas sean satisfechas.
// Si la lista está vacía, siempre retorna true (vacuously true).
func AllRules(rs ...Rule) Rule {
	return &allRules{rs: rs}
}

func (a *allRules) Satisfies(entry vos.HistoryEntry, current vos.FingerprintSet, now time.Time) bool {
	for _, r := range a.rs {
		if !r.Satisfies(entry, current, now) {
			return false
		}
	}
	return true
}
