package usecase

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/application"
	exePrt "github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
	exeVos "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
)

// StreamExecutionLogsUseCase retorna un canal de strings que el HTTP handler puede
// consumir para hacer SSE streaming de logs. Si la ejecución ya terminó, retorna
// un canal cerrado con un mensaje de estado final.
type StreamExecutionLogsUseCase struct {
	broker *application.MemLogBroker
	repo   exePrt.ExecutionRepository
}

// NewStreamExecutionLogsUseCase construye el use case con el broker y repositorio inyectados.
func NewStreamExecutionLogsUseCase(broker *application.MemLogBroker, repo exePrt.ExecutionRepository) *StreamExecutionLogsUseCase {
	return &StreamExecutionLogsUseCase{broker: broker, repo: repo}
}

// Execute retorna un canal de strings. El caller debe drenarlo hasta que se cierre.
// Si la ejecución ya terminó (status terminal), retorna un canal ya cerrado con
// un mensaje informativo.
func (uc *StreamExecutionLogsUseCase) Execute(ctx context.Context, executionID string) (<-chan string, error) {
	id, err := exeVos.ExecutionIDFromString(executionID)
	if err != nil {
		return nil, fmt.Errorf("use case stream execution logs: %w", err)
	}

	execution, err := uc.repo.FindByID(ctx, id)
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
	}

	return uc.broker.Subscribe(id), nil
}
