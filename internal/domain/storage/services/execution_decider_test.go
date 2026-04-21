package services

import (
	"context"
	"errors"
	"testing"
	"time"

	storageDomain "github.com/jairoprogramador/vex-engine/internal/domain/storage"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/aggregates"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

// --- Mocks ---

type mockRepo struct {
	history *aggregates.ExecutionHistory
	err     error
	saved   *aggregates.ExecutionHistory
}

func (m *mockRepo) FindByKey(_ context.Context, _ vos.StorageKey) (*aggregates.ExecutionHistory, error) {
	return m.history, m.err
}

func (m *mockRepo) Save(_ context.Context, h *aggregates.ExecutionHistory) error {
	m.saved = h
	return nil
}

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

// --- Helpers ---

func mustFPt(v string) vos.Fingerprint {
	fp, err := vos.NewFingerprint(v)
	if err != nil {
		panic(err)
	}
	return fp
}

func mustEnvt(v string) vos.Environment {
	e, err := vos.NewEnvironment(v)
	if err != nil {
		panic(err)
	}
	return e
}

func testStorageKey() vos.StorageKey {
	return vos.NewStorageKey("proj", "tmpl", vos.StepTest)
}

func testFingerprintSet() vos.FingerprintSet {
	return vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
		vos.KindCode:        mustFPt("code123"),
		vos.KindInstruction: mustFPt("inst123"),
		vos.KindVars:        mustFPt("vars123"),
	}, mustEnvt("sand"))
}

// --- Tests ---

func TestExecutionDecider_Decide_EmptyHistory_ReturnsRun(t *testing.T) {
	repo := &mockRepo{history: nil, err: nil}
	clock := &fakeClock{t: time.Now()}
	decider := NewExecutionDecider(repo, DefaultCatalog(), clock)

	decision, err := decider.Decide(context.Background(), testStorageKey(), testFingerprintSet())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !decision.ShouldRun() {
		t.Error("historial vacío debe retornar Run")
	}
}

func TestExecutionDecider_Decide_CorruptedHistory_ReturnsRunNoError(t *testing.T) {
	repo := &mockRepo{err: storageDomain.ErrHistoryCorrupted}
	clock := &fakeClock{t: time.Now()}
	decider := NewExecutionDecider(repo, DefaultCatalog(), clock)

	decision, err := decider.Decide(context.Background(), testStorageKey(), testFingerprintSet())
	if err != nil {
		t.Fatalf("historial corrupto no debe propagarse como error: %v", err)
	}
	if !decision.ShouldRun() {
		t.Error("historial corrupto debe retornar Run")
	}
}

func TestExecutionDecider_Decide_RepoError_Propagates(t *testing.T) {
	repoErr := errors.New("connection refused")
	repo := &mockRepo{err: repoErr}
	clock := &fakeClock{t: time.Now()}
	decider := NewExecutionDecider(repo, DefaultCatalog(), clock)

	_, err := decider.Decide(context.Background(), testStorageKey(), testFingerprintSet())
	if err == nil {
		t.Fatal("error de repo debe propagarse")
	}
	if !errors.Is(err, repoErr) {
		t.Errorf("error esperado contener %v, obtuvo %v", repoErr, err)
	}
}

func TestExecutionDecider_Decide_MatchingEntry_ReturnsSkip(t *testing.T) {
	now := time.Now().UTC()
	current := testFingerprintSet()

	// Crear historial con una entrada que coincide exactamente con el fingerprint set actual
	h := aggregates.NewExecutionHistory(testStorageKey())
	// Añadir entrada con los mismos fingerprints — la política de test requiere instruction+vars+code+TTL
	matchSet := vos.NewFingerprintSet(map[vos.FingerprintKind]vos.Fingerprint{
		vos.KindCode:        mustFPt("code123"),
		vos.KindInstruction: mustFPt("inst123"),
		vos.KindVars:        mustFPt("vars123"),
	}, mustEnvt("sand"))
	h.Append(matchSet, now.Add(-1*time.Hour)) // reciente, dentro del TTL de 30d

	repo := &mockRepo{history: h}
	clock := &fakeClock{t: now}
	decider := NewExecutionDecider(repo, DefaultCatalog(), clock)

	decision, err := decider.Decide(context.Background(), testStorageKey(), current)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if decision.ShouldRun() {
		t.Error("entrada coincidente debe retornar Skip")
	}
}

func TestExecutionDecider_RecordSuccess_PersistsEntry(t *testing.T) {
	repo := &mockRepo{history: nil}
	clock := &fakeClock{t: time.Now()}
	decider := NewExecutionDecider(repo, DefaultCatalog(), clock)

	err := decider.RecordSuccess(context.Background(), testStorageKey(), testFingerprintSet())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if repo.saved == nil {
		t.Error("debe haber guardado el historial")
	}
	if len(repo.saved.Entries()) != 1 {
		t.Errorf("debe haber guardado 1 entrada, tiene %d", len(repo.saved.Entries()))
	}
}
