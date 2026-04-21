package rules

import (
	"testing"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

func TestTTLRule(t *testing.T) {
	ttl := vos.NewTTL(24 * time.Hour)
	rule := NewTTLRule(ttl)

	t.Run("entrada reciente no expirada → true", func(t *testing.T) {
		entry := &stubEntry{createdAt: time.Now().Add(-1 * time.Hour)}
		current := vos.NewFingerprintSet(nil, mustEnv("sand"))
		if !rule.Satisfies(entry, current, time.Now()) {
			t.Error("entrada reciente debe satisfacer la regla TTL")
		}
	})

	t.Run("entrada expirada → false", func(t *testing.T) {
		entry := &stubEntry{createdAt: time.Now().Add(-25 * time.Hour)}
		current := vos.NewFingerprintSet(nil, mustEnv("sand"))
		if rule.Satisfies(entry, current, time.Now()) {
			t.Error("entrada expirada no debe satisfacer la regla TTL")
		}
	})

	t.Run("exactamente en el límite de expiración → false", func(t *testing.T) {
		// now.Before(expiration) → false cuando son iguales
		createdAt := time.Now().Add(-24 * time.Hour)
		entry := &stubEntry{createdAt: createdAt}
		current := vos.NewFingerprintSet(nil, mustEnv("sand"))
		now := createdAt.Add(ttl.Duration())
		if rule.Satisfies(entry, current, now) {
			t.Error("exactamente en el límite debe retornar false (Before es estricto)")
		}
	})
}
