package usecase

import (
	"context"
	"fmt"
	"time"

	exePrt "github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
	exeVos "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
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
	repo exePrt.ExecutionRepository
}

// NewGetExecutionUseCase construye el use case con el repositorio inyectado.
func NewGetExecutionUseCase(repo exePrt.ExecutionRepository) *GetExecutionUseCase {
	return &GetExecutionUseCase{repo: repo}
}

// Execute busca la ejecución por ID y retorna su vista de lectura.
// Retorna error si el ID es inválido o si la ejecución no existe.
func (uc *GetExecutionUseCase) Execute(ctx context.Context, executionID string) (ExecutionView, error) {
	id, err := exeVos.ExecutionIDFromString(executionID)
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
		ProjectID:   execution.ProjectID(),
		ProjectName: execution.ProjectName(),
		StartedAt:   execution.StartedAt(),
		FinishedAt:  execution.FinishedAt(),
		ExitCode:    execution.ExitCode(),
	}, nil
}
