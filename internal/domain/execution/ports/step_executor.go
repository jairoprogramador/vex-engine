package ports

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/execution/entities"
	"github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
)

// StepExecutor define la interfaz para ejecutar un único paso de un plan de ejecución.
type StepExecutor interface {
	Execute(
		ctx context.Context,
		step *entities.Step,
		initialVars vos.VariableSet,
		emitter LogEmitter,
		executionID vos.ExecutionID,
	) (*vos.ExecutionResult, error)
}
