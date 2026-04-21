package rules

import (
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

type environmentRule struct{}

// NewEnvironmentRule retorna una Rule que verifica que el entorno de la entrada
// histórica coincida con el del estado actual, usando Equals en lugar de == string.
func NewEnvironmentRule() Rule {
	return &environmentRule{}
}

func (r *environmentRule) Satisfies(entry vos.HistoryEntry, current vos.FingerprintSet, _ time.Time) bool {
	return entry.Environment().Equals(current.Environment())
}
