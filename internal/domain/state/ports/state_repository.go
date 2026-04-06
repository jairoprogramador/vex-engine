package ports

import (
	"github.com/jairoprogramador/vex-engine/internal/domain/state/aggregates"
)

type StateRepository interface {
	Get(filePath string) (*aggregates.StateTable, error)
	Save(filePath string, stateTable *aggregates.StateTable) error
}
