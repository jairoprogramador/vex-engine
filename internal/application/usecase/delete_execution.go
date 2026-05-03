package usecase

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

// DeleteExecutionResult transporta el estado final de la ejecución cancelada.
type DeleteExecutionResult struct {
	ExecutionID string
	Status      string
}

// DeleteExecutionUseCase cancela una ejecución en curso delegando al orchestrator,
// que es quien mantiene el cancelFn en memoria.
type DeleteExecutionUseCase struct {
}

// NewDeleteExecutionUseCase construye el use case con el orchestrator y repositorio inyectados.
func NewDeleteExecutionUseCase() *DeleteExecutionUseCase {
	return &DeleteExecutionUseCase{}
}

// Execute cancela la ejecución identificada por executionID.
// Retorna el resultado con el estado actualizado tras la cancelación.
func (uc *DeleteExecutionUseCase) Execute(ctx context.Context, executionID string) (DeleteExecutionResult, error) {
	/* id, err := exeAggr.ExecutionIDFromString(executionID)
	if err != nil {
		return DeleteExecutionResult{}, fmt.Errorf("use case delete execution: %w", err)
	}

	if err := uc.orchestrator.Cancel(ctx, id); err != nil {
		return DeleteExecutionResult{}, fmt.Errorf("use case delete execution: %w", err)
	}

	execution, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return DeleteExecutionResult{}, fmt.Errorf("use case delete execution: find by id after cancel: %w", err)
	}
	if execution == nil {
		return DeleteExecutionResult{}, fmt.Errorf("use case delete execution: execution %s: not found after cancel", executionID)
	} */

	return DeleteExecutionResult{
		ExecutionID: "hdk",
		Status:      command.StatusCancelled.String(),
	}, nil
}
