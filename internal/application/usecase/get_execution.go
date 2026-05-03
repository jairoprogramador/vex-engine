package usecase

import (
	"context"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

// ExecutionView es la proyección de lectura del agregado Execution.
type ExecutionView struct {
	ExecutionID string
	Status      string
	Step        string
	Environment string
	ProjectID   string
	ProjectName string
	StartedAt   time.Time
	FinishedAt  *time.Time
	ExitCode    *int
}

// GetExecutionUseCase consulta el repositorio por ID y retorna una vista completa.
type GetExecutionUseCase struct {
}

// NewGetExecutionUseCase construye el use case con el repositorio inyectado.
func NewGetExecutionUseCase() *GetExecutionUseCase {
	return &GetExecutionUseCase{}
}

// Execute busca la ejecución por ID y retorna su vista de lectura.
// Retorna error si el ID es inválido o si la ejecución no existe.
func (uc *GetExecutionUseCase) Execute(ctx context.Context, executionID string) (ExecutionView, error) {
	/* id, err := exeAggr.ExecutionIDFromString(executionID)
	if err != nil {
		return ExecutionView{}, fmt.Errorf("use case get execution: %w", err)
	}

	execution, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return ExecutionView{}, fmt.Errorf("use case get execution: %w", err)
	}
	if execution == nil {
		return ExecutionView{}, fmt.Errorf("use case get execution: execution %s: not found", executionID)
	}

	return ExecutionView{
		ExecutionID: execution.ID().String(),
		Status:      execution.Status().String(),
		Step:        execution.Step(),
		Environment: execution.Environment(),
		ProjectID:   execution.ProjectId(),
		ProjectName: execution.ProjectName(),
		StartedAt:   execution.StartedAt(),
		FinishedAt:  execution.FinishedAt(),
		ExitCode:    execution.ExitCode(),
	}, nil */
	return ExecutionView{
		ExecutionID: "hdk",
		Status:      command.StatusCancelled.String(),
		Step:        "hdk",
		Environment: "hdk",
		ProjectID:   "hdk",
		ProjectName: "hdk",
		StartedAt:   time.Now(),
		FinishedAt:  nil,
		ExitCode:    nil,
	}, nil
}
