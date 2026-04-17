package usecase

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/application"
	exePrt "github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
	exeVos "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
)

// DeleteExecutionResult transporta el estado final de la ejecución cancelada.
type DeleteExecutionResult struct {
	ExecutionID string
	Status      string
}

// DeleteExecutionUseCase cancela una ejecución en curso delegando al orchestrator,
// que es quien mantiene el cancelFn en memoria.
type DeleteExecutionUseCase struct {
	orchestrator *application.ExecutionOrchestrator
	repo         exePrt.ExecutionRepository
}

// NewDeleteExecutionUseCase construye el use case con el orchestrator y repositorio inyectados.
func NewDeleteExecutionUseCase(orchestrator *application.ExecutionOrchestrator, repo exePrt.ExecutionRepository) *DeleteExecutionUseCase {
	return &DeleteExecutionUseCase{orchestrator: orchestrator, repo: repo}
}

// Execute cancela la ejecución identificada por executionID.
// Retorna el resultado con el estado actualizado tras la cancelación.
func (uc *DeleteExecutionUseCase) Execute(ctx context.Context, executionID string) (DeleteExecutionResult, error) {
	id, err := exeVos.ExecutionIDFromString(executionID)
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
	}

	return DeleteExecutionResult{
		ExecutionID: execution.ID().String(),
		Status:      execution.Status().String(),
	}, nil
}
