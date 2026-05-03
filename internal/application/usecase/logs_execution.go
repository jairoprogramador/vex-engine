package usecase

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/notify"
)

// LogsExecutionUseCase retorna un canal de strings que el HTTP handler puede
// consumir para hacer SSE streaming de logs. Si la ejecución ya terminó, retorna
// un canal cerrado con un mensaje de estado final.
type LogsExecutionUseCase struct {
	broker *notify.MemLogPublisher
}

// NewLogsExecutionUseCase construye el use case con el broker y repositorio inyectados.
func NewLogsExecutionUseCase(broker *notify.MemLogPublisher) *LogsExecutionUseCase {
	return &LogsExecutionUseCase{broker: broker}
}

// Execute retorna un canal de strings. El caller debe drenarlo hasta que se cierre.
// Si la ejecución ya terminó (status terminal), retorna un canal ya cerrado con
// un mensaje informativo.
func (uc *LogsExecutionUseCase) Execute(ctx context.Context, executionID string) (<-chan string, error) {
	parsedExecutionID, err := command.ExecutionIDFromString(executionID)
	if err != nil {
		return nil, fmt.Errorf("use case stream execution logs: %w", err)
	}

	/* execution, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("use case stream execution logs: %w", err)
	}
	if execution == nil {
		return nil, fmt.Errorf("use case stream execution logs: execution %s: not found", executionID)
	}

	if execution.Status().IsTerminal() {
		ch := make(chan string, 1)
		ch <- fmt.Sprintf("execution already finished with status: %s", execution.Status().String())
		close(ch)
		return ch, nil
	} */

	return uc.broker.Subscribe(parsedExecutionID), nil
}
