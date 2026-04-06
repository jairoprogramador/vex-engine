package ports

import (
	"github.com/jairoprogramador/vex-engine/internal/domain/state/aggregates"
	"github.com/jairoprogramador/vex-engine/internal/domain/state/vos"
)

type StateMatcher interface {
	Match(entry *aggregates.StateEntry, current vos.CurrentStateFingerprints) bool
}
