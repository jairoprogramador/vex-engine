package ports

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/execution/aggregates"
	"github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
)

// ExecutionRepository define el contrato de persistencia para el agregado Execution.
// Las implementaciones concretas viven en infrastructure/storage.
type ExecutionRepository interface {
	Save(ctx context.Context, execution *aggregates.Execution) error
	FindByID(ctx context.Context, id vos.ExecutionID) (*aggregates.Execution, error)
	UpdateStatus(ctx context.Context, id vos.ExecutionID, status vos.ExecutionStatus) error
}
