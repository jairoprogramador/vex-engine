package ports

import (
	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/domain/logger/aggregates"
)

type LoggerRepository interface {
	Save(namesParams dto.NamesParams, logger *aggregates.Logger) error
	Find(namesParams dto.NamesParams) (aggregates.Logger, error)
}
