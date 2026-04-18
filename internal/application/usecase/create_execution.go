package usecase

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/application"
	"github.com/jairoprogramador/vex-engine/internal/application/dto"
)

// CreateExecutionOutput transporta el resultado inmediato de enqueue de una ejecución.
type CreateExecutionOutput struct {
	ExecutionID string
	Status      string
}

// CreateExecutionUseCase valida el request mínimo y delega al ExecutionOrchestrator.
type CreateExecutionUseCase struct {
	orchestrator *application.ExecutionOrchestrator
}

// NewCreateExecutionUseCase construye el use case con el orchestrator inyectado.
func NewCreateExecutionUseCase(orchestrator *application.ExecutionOrchestrator) *CreateExecutionUseCase {
	return &CreateExecutionUseCase{orchestrator: orchestrator}
}

// Execute valida el comando, lanza la ejecución de forma no bloqueante y retorna
// el ID asignado con estado "queued".
func (uc *CreateExecutionUseCase) Execute(ctx context.Context, cmd dto.RequestInput) (CreateExecutionOutput, error) {
	if cmd.Execution.Step == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: step is required")
	}
	if cmd.Execution.Environment == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: environment is required")
	}

	executionID, err := uc.orchestrator.Run(ctx, cmd)
	if err != nil {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: %w", err)
	}

	return CreateExecutionOutput{
		ExecutionID: executionID.String(),
		Status:      "queued",
	}, nil
}
