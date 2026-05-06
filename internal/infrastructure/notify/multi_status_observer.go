package notify

import (
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

// MultiStatusObserver es el análogo a MultiObserver para transiciones de stage.
// Replica cada Notify a sus observers concretos en orden de registro.
type MultiStatusObserver struct {
	observers []domNotify.StatusObserver
}

func NewMultiStatusObserver(observers ...domNotify.StatusObserver) *MultiStatusObserver {
	filtered := make([]domNotify.StatusObserver, 0, len(observers))
	for _, o := range observers {
		if o != nil {
			filtered = append(filtered, o)
		}
	}
	return &MultiStatusObserver{observers: filtered}
}

func (m *MultiStatusObserver) Notify(executionID string, stage string) {
	for _, o := range m.observers {
		o.Notify(executionID, stage)
	}
}

var _ domNotify.StatusObserver = (*MultiStatusObserver)(nil)
