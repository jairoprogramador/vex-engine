package rules

import (
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

type ttlRule struct {
	ttl vos.TTL
}

// NewTTLRule retorna una Rule que verifica que la entrada histórica no haya expirado
// según el TTL dado en relación al momento now.
func NewTTLRule(ttl vos.TTL) Rule {
	return &ttlRule{ttl: ttl}
}

func (r *ttlRule) Satisfies(entry vos.HistoryEntry, _ vos.FingerprintSet, now time.Time) bool {
	expiration := entry.CreatedAt().Add(r.ttl.Duration())
	return now.Before(expiration)
}
