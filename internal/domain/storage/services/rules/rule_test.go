package rules

import (
	"testing"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

// stubEntry es una implementación mínima de vos.HistoryEntry para tests.
type stubEntry struct {
	fingerprints map[vos.FingerprintKind]vos.Fingerprint
	environment  vos.Environment
	createdAt    time.Time
}

func (s *stubEntry) FindFingerprintByKind(kind vos.FingerprintKind) (vos.Fingerprint, bool) {
	fp, ok := s.fingerprints[kind]
	return fp, ok
}

func (s *stubEntry) Environment() vos.Environment { return s.environment }
func (s *stubEntry) CreatedAt() time.Time         { return s.createdAt }

func mustFP(v string) vos.Fingerprint {
	fp, err := vos.NewFingerprint(v)
	if err != nil {
		panic(err)
	}
	return fp
}

func mustEnv(v string) vos.Environment {
	e, err := vos.NewEnvironment(v)
	if err != nil {
		panic(err)
	}
	return e
}

func TestAllRules_EmptyAlwaysTrue(t *testing.T) {
	rule := AllRules()
	entry := &stubEntry{}
	current := vos.NewFingerprintSet(nil, mustEnv("sand"))
	if !rule.Satisfies(entry, current, time.Now()) {
		t.Error("AllRules vacío debe retornar true")
	}
}

func TestAllRules_CutsAtFirstFalse(t *testing.T) {
	called := 0
	// Segunda regla que cuenta llamadas
	second := &countingRule{count: &called}

	// Primera regla siempre false
	first := alwaysFalse{}

	rule := AllRules(first, second)
	entry := &stubEntry{}
	current := vos.NewFingerprintSet(nil, mustEnv("sand"))

	result := rule.Satisfies(entry, current, time.Now())
	if result {
		t.Error("AllRules debe retornar false cuando la primera regla falla")
	}
	if called != 0 {
		t.Errorf("la segunda regla no debe evaluarse si la primera falla; se llamó %d veces", called)
	}
}

type alwaysFalse struct{}

func (alwaysFalse) Satisfies(_ vos.HistoryEntry, _ vos.FingerprintSet, _ time.Time) bool {
	return false
}

type countingRule struct{ count *int }

func (r *countingRule) Satisfies(_ vos.HistoryEntry, _ vos.FingerprintSet, _ time.Time) bool {
	*r.count++
	return true
}
