package ports

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/aggregates"
	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

type HistoryRepository interface {
	FindByKey(ctx context.Context, key vos.StorageKey) (*aggregates.ExecutionHistory, error)
	Save(ctx context.Context, history *aggregates.ExecutionHistory) error
}
