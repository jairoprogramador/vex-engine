package ports

import (
	"github.com/jairoprogramador/vex/internal/domain/state/vos"
)

type StateManager interface {
	HasStateChanged(
		stateTablePath string,
		currentState vos.CurrentStateFingerprints,
		policy vos.CachePolicy,
	) (bool, error)

	UpdateState(stateTablePath string, currentState vos.CurrentStateFingerprints) error
}
