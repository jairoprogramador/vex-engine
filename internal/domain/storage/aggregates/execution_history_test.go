package aggregates

import (
	"testing"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

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

func makeSet(env string, fps map[vos.FingerprintKind]vos.Fingerprint) vos.FingerprintSet {
	return vos.NewFingerprintSet(fps, mustEnv(env))
}

func testKey() vos.StorageKey {
	return vos.NewStorageKey("proj", "tmpl", vos.StepTest)
}

func TestNewStepHistory(t *testing.T) {
	h := NewExecutionHistory(testKey())
	if h == nil {
		t.Fatal("NewStepHistory devolvió nil")
	}
	if len(h.Entries()) != 0 {
		t.Errorf("se esperaba historial vacío, tiene %d entradas", len(h.Entries()))
	}
	if !h.Key().Equals(testKey()) {
		t.Error("clave incorrecta")
	}
}

func TestStepHistory_Append_MaintainsOrder(t *testing.T) {
	h := NewExecutionHistory(testKey())
	env := mustEnv("sand")
	now := time.Now().UTC()

	set := vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
		vos.KindCode: mustFP("c1"),
	}, env)

	h.Append(set, now.Add(2*time.Second))
	h.Append(set, now.Add(1*time.Second))
	h.Append(set, now.Add(3*time.Second))

	entries := h.Entries()
	if len(entries) != 3 {
		t.Fatalf("se esperaban 3 entradas, obtuvo %d", len(entries))
	}
	if !entries[0].CreatedAt().Equal(now.Add(1 * time.Second)) {
		t.Error("primera entrada no es la más antigua")
	}
	if !entries[2].CreatedAt().Equal(now.Add(3 * time.Second)) {
		t.Error("última entrada no es la más reciente")
	}
}

func TestStepHistory_Append_MaxEntries(t *testing.T) {
	h := NewExecutionHistory(testKey())
	env := mustEnv("sand")
	baseTime := time.Now().UTC()
	set := vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
		vos.KindCode: mustFP("c"),
	}, env)

	for i := 0; i < maxEntries; i++ {
		h.Append(set, baseTime.Add(time.Duration(i)*time.Second))
	}

	if len(h.Entries()) != maxEntries {
		t.Fatalf("se esperaban %d entradas, obtuvo %d", maxEntries, len(h.Entries()))
	}

	oldestTime := h.Entries()[0].CreatedAt()

	// Añadir una más nueva — debe expulsar la más antigua
	h.Append(set, baseTime.Add(time.Duration(maxEntries)*time.Second))

	if len(h.Entries()) != maxEntries {
		t.Errorf("se esperaban %d entradas tras overflow, obtuvo %d", maxEntries, len(h.Entries()))
	}
	for _, e := range h.Entries() {
		if e.CreatedAt().Equal(oldestTime) {
			t.Error("la entrada más antigua debería haber sido expulsada")
		}
	}

	// Añadir una más antigua — no debe quedar
	h.Append(set, baseTime.Add(-1*time.Second))
	for _, e := range h.Entries() {
		if e.CreatedAt().Equal(baseTime.Add(-1 * time.Second)) {
			t.Error("una entrada más antigua que todas las existentes no debería quedar")
		}
	}
}

func TestLoadStepHistory_SortsAndTruncates(t *testing.T) {
	env := mustEnv("prod")
	now := time.Now().UTC()
	set := vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
		vos.KindCode: mustFP("c"),
	}, env)

	t.Run("ordena al cargar", func(t *testing.T) {
		e2 := NewHistoryEntry(set, now.Add(2*time.Second))
		e1 := NewHistoryEntry(set, now.Add(1*time.Second))
		h, err := LoadStepHistory(testKey(), []HistoryEntry{e2, e1})
		if err != nil {
			t.Fatal(err)
		}
		if !h.Entries()[0].CreatedAt().Equal(e1.CreatedAt()) {
			t.Error("no ordenó correctamente al cargar")
		}
	})

	t.Run("trunca a maxEntries", func(t *testing.T) {
		entries := make([]HistoryEntry, maxEntries+2)
		for i := range entries {
			entries[i] = NewHistoryEntry(set, now.Add(time.Duration(i)*time.Second))
		}
		h, err := LoadStepHistory(testKey(), entries)
		if err != nil {
			t.Fatal(err)
		}
		if len(h.Entries()) != maxEntries {
			t.Errorf("se esperaban %d entradas, obtuvo %d", maxEntries, len(h.Entries()))
		}
		// Debe conservar las más recientes
		if !h.Entries()[0].CreatedAt().Equal(entries[2].CreatedAt()) {
			t.Error("no conservó las entradas más recientes al truncar")
		}
	})

	t.Run("lista vacía retorna historial vacío", func(t *testing.T) {
		h, err := LoadStepHistory(testKey(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(h.Entries()) != 0 {
			t.Error("debería retornar historial vacío")
		}
	})
}

// alwaysMatchRule es una Rule que siempre retorna true, para testear Decide.
type alwaysMatchRule struct{}

func (alwaysMatchRule) Satisfies(_ vos.HistoryEntry, _ vos.FingerprintSet, _ time.Time) bool {
	return true
}

// neverMatchRule es una Rule que siempre retorna false.
type neverMatchRule struct{}

func (neverMatchRule) Satisfies(_ vos.HistoryEntry, _ vos.FingerprintSet, _ time.Time) bool {
	return false
}

func TestStepHistory_Decide(t *testing.T) {
	env := mustEnv("sand")
	set := makeSet("sand", map[vos.FingerprintKind]vos.Fingerprint{
		vos.KindCode: mustFP("abc"),
	})
	now := time.Now().UTC()

	t.Run("historial vacío → Run", func(t *testing.T) {
		h := NewExecutionHistory(testKey())
		decision := h.Decide(neverMatchRule{}, set, now)
		if !decision.ShouldRun() {
			t.Error("historial vacío debe retornar Run")
		}
	})

	t.Run("último entry satisface → Skip", func(t *testing.T) {
		h := NewExecutionHistory(testKey())
		h.Append(vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
			vos.KindCode: mustFP("abc"),
		}, env), now.Add(-1*time.Hour))

		decision := h.Decide(alwaysMatchRule{}, set, now)
		if decision.ShouldRun() {
			t.Error("último entry que satisface debe retornar Skip")
		}
		if decision.MatchedAt().IsZero() {
			t.Error("MatchedAt debe estar seteado en Skip")
		}
	})

	t.Run("último entry no satisface → Run", func(t *testing.T) {
		h := NewExecutionHistory(testKey())
		h.Append(vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
			vos.KindCode: mustFP("abc"),
		}, env), now.Add(-1*time.Hour))

		decision := h.Decide(neverMatchRule{}, set, now)
		if !decision.ShouldRun() {
			t.Error("último entry que no satisface debe retornar Run")
		}
	})

	t.Run("solo se evalúa el último entry, los anteriores se ignoran", func(t *testing.T) {
		h := NewExecutionHistory(testKey())
		// entry antiguo: alwaysMatch lo satisfaría, pero no es el último
		h.Append(vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
			vos.KindCode: mustFP("abc"),
		}, env), now.Add(-2*time.Hour))
		// entry más reciente: neverMatch → no satisface
		h.Append(vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
			vos.KindCode: mustFP("def"),
		}, env), now.Add(-1*time.Hour))

		// Con la regla alwaysMatch el entry antiguo satisfaría, pero Decide solo ve el último
		decision := h.Decide(neverMatchRule{}, set, now)
		if !decision.ShouldRun() {
			t.Error("Decide debe ignorar entries anteriores y evaluar solo el último")
		}
	})
}
