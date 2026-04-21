package aggregates

import (
	"fmt"
	"sort"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

const maxEntries = 10

type ExecutionHistory struct {
	key     vos.StorageKey
	entries []HistoryEntry
}

func NewExecutionHistory(key vos.StorageKey) *ExecutionHistory {
	return &ExecutionHistory{
		key:     key,
		entries: make([]HistoryEntry, 0, maxEntries),
	}
}

func LoadStepHistory(key vos.StorageKey, entries []HistoryEntry) (*ExecutionHistory, error) {
	if len(entries) == 0 {
		return NewExecutionHistory(key), nil
	}

	sorted := make([]HistoryEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt().Before(sorted[j].CreatedAt())
	})

	if len(sorted) > maxEntries {
		sorted = sorted[len(sorted)-maxEntries:]
	}

	return &ExecutionHistory{key: key, entries: sorted}, nil
}

func (h *ExecutionHistory) Append(set vos.FingerprintSet, now time.Time) {
	entry := NewHistoryEntry(set, now)

	idx := sort.Search(len(h.entries), func(i int) bool {
		return h.entries[i].CreatedAt().After(entry.CreatedAt())
	})

	h.entries = append(h.entries[:idx], append([]HistoryEntry{entry}, h.entries[idx:]...)...)

	if len(h.entries) > maxEntries {
		h.entries = h.entries[len(h.entries)-maxEntries:]
	}
}

func (h *ExecutionHistory) Decide(rule vos.Rule, current vos.FingerprintSet, now time.Time) vos.Decision {
	if len(h.entries) == 0 {
		return vos.DecisionRun("sin historial de ejecución")
	}
	last := h.entries[len(h.entries)-1]
	if rule.Satisfies(last, current, now) {
		return vos.DecisionSkip(last.CreatedAt())
	}
	return vos.DecisionRun("la última ejecución no satisface la política actual")
}

func (h *ExecutionHistory) Entries() []HistoryEntry { return h.entries }
func (h *ExecutionHistory) Key() vos.StorageKey     { return h.key }

func validateKey(key vos.StorageKey) error {
	if key.ProjectName() == "" || key.TemplateName() == "" {
		return fmt.Errorf("storage key inválida: projectName y templateName no pueden estar vacíos")
	}
	return nil
}

var _ = validateKey // supress unused warning
