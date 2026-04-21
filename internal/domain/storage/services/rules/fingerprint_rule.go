package rules

import (
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

type fingerprintRule struct {
	kind vos.FingerprintKind
}

// NewFingerprintRule retorna una Rule que verifica que el fingerprint del kind
// dado sea idéntico en la entrada histórica y en el estado actual.
// Si alguno de los dos está ausente, la regla no se satisface.
func NewFingerprintRule(kind vos.FingerprintKind) Rule {
	return &fingerprintRule{kind: kind}
}

func (r *fingerprintRule) Satisfies(entry vos.HistoryEntry, current vos.FingerprintSet, _ time.Time) bool {
	entryFp, entryOk := entry.FindFingerprintByKind(r.kind)
	currentFp, currentOk := current.Get(r.kind)
	if !entryOk || !currentOk {
		return false
	}
	return entryFp.Equals(currentFp)
}
