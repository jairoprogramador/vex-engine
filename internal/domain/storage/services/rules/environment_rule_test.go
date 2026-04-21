package rules

import (
	"testing"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

func TestEnvironmentRule(t *testing.T) {
	rule := NewEnvironmentRule()
	now := time.Now()

	t.Run("mismo entorno → true", func(t *testing.T) {
		entry := &stubEntry{environment: mustEnv("prod")}
		current := vos.NewFingerprintSet(nil, mustEnv("prod"))
		if !rule.Satisfies(entry, current, now) {
			t.Error("mismo entorno debe satisfacer la regla")
		}
	})

	t.Run("entornos distintos → false", func(t *testing.T) {
		entry := &stubEntry{environment: mustEnv("prod")}
		current := vos.NewFingerprintSet(nil, mustEnv("sand"))
		if rule.Satisfies(entry, current, now) {
			t.Error("entornos distintos no deben satisfacer la regla")
		}
	})
}
