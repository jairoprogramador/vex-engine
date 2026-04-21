package rules

import (
	"testing"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

func TestFingerprintRule(t *testing.T) {
	rule := NewFingerprintRule(vos.KindCode)
	now := time.Now()

	t.Run("fingerprints iguales → true", func(t *testing.T) {
		entry := &stubEntry{
			fingerprints: map[vos.FingerprintKind]vos.Fingerprint{vos.KindCode: mustFP("abc")},
		}
		current := vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
			vos.KindCode: mustFP("abc"),
		}, mustEnv("sand"))
		if !rule.Satisfies(entry, current, now) {
			t.Error("fingerprints iguales deben satisfacer la regla")
		}
	})

	t.Run("fingerprints distintos → false", func(t *testing.T) {
		entry := &stubEntry{
			fingerprints: map[vos.FingerprintKind]vos.Fingerprint{vos.KindCode: mustFP("abc")},
		}
		current := vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
			vos.KindCode: mustFP("xyz"),
		}, mustEnv("sand"))
		if rule.Satisfies(entry, current, now) {
			t.Error("fingerprints distintos no deben satisfacer la regla")
		}
	})

	t.Run("fingerprint ausente en entry → false", func(t *testing.T) {
		entry := &stubEntry{fingerprints: map[vos.FingerprintKind]vos.Fingerprint{}}
		current := vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
			vos.KindCode: mustFP("abc"),
		}, mustEnv("sand"))
		if rule.Satisfies(entry, current, now) {
			t.Error("ausencia en entry debe retornar false")
		}
	})

	t.Run("fingerprint ausente en current → false", func(t *testing.T) {
		entry := &stubEntry{
			fingerprints: map[vos.FingerprintKind]vos.Fingerprint{vos.KindCode: mustFP("abc")},
		}
		current := vos.NewFingerprintSet(nil, mustEnv("sand"))
		if rule.Satisfies(entry, current, now) {
			t.Error("ausencia en current debe retornar false")
		}
	})
}
