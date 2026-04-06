package ports

import (
	"github.com/jairoprogramador/vex-engine/internal/domain/state/vos"
)

type StateManager interface {
	HasStateChanged(
		stateTablePath string,
		currentState vos.CurrentStateFingerprints,
		policy vos.CachePolicy,
	) (bool, error)

	UpdateState(stateTablePath string, currentState vos.CurrentStateFingerprints) error
}
