package usecase

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/notify"
	//"github.com/jairoprogramador/vex-engine/internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

// CreateExecutionOutput transporta el resultado inmediato de enqueue de una ejecución.
type CreateExecutionOutput struct {
	ExecutionID string
	Status      string
}

// CreateExecutionUseCase valida el request mínimo y delega al ExecutionOrchestrator.
type CreateExecutionUseCase struct {
	executablePipeline command.Executable
	executableCommand  command.Executable
	executableStep     command.Executable
	notify             *notify.MemLogPublisher
}

// NewCreateExecutionUseCase construye el use case con el orchestrator inyectado.
func NewCreateExecutionUseCase(
	executablePipeline command.Executable,
	executableCommand command.Executable,
	executableStep command.Executable,
	notify *notify.MemLogPublisher) *CreateExecutionUseCase {
	return &CreateExecutionUseCase{
		executablePipeline: executablePipeline,
		executableCommand:  executableCommand,
		executableStep:     executableStep,
		notify:             notify,
	}
}

// Execute valida el comando, lanza la ejecución de forma no bloqueante y retorna
// el ID asignado con estado "queued".
func (uc *CreateExecutionUseCase) Execute(ctx context.Context, request dto.RequestInput) (CreateExecutionOutput, error) {
	if request.Execution.Step == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: step is required")
	}
	if request.Execution.Environment == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: environment is required")
	}
	if request.Project.Id == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: project id is required")
	}
	if request.Project.Name == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: project name is required")
	}
	if request.Project.Team == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: project team is required")
	}
	if request.Project.Org == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: project org is required")
	}
	if request.Project.Url == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: project url is required")
	}
	if request.Project.Ref == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: project ref is required")
	}
	if request.Pipeline.Url == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: pipeline url is required")
	}
	if request.Pipeline.Ref == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: pipeline ref is required")
	}

	projectUrl, err := shared.NewRepositoryURL(request.Project.Url)
	if err != nil {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: project url is invalid: %w", err)
	}

	pipelineUrl, err := shared.NewRepositoryURL(request.Pipeline.Url)
	if err != nil {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: pipeline url is invalid: %w", err)
	}

	execution := command.NewExecution(
		command.NewExecutionProject(request.Project.Id, request.Project.Name, projectUrl.String(), request.Project.Ref, request.Project.Org, request.Project.Team),
		command.NewExecutionPipeline(pipelineUrl.String(), request.Pipeline.Ref),
		request.Execution.Step,
		request.Execution.Environment,
		command.NewExecutionRuntime(request.Execution.RuntimeImage, request.Execution.RuntimeTag),
	)

	childCtx, cancelFn := context.WithCancel(context.Background())
	execution.SetCancelFn(cancelFn)

	executionContext := command.NewExecutionContext(
		&childCtx,
		execution,
		uc.executableCommand,
		uc.executableStep,
		uc.notify,
	)

	err = uc.executablePipeline.Execute(executionContext)
	if err != nil {
		return CreateExecutionOutput{}, fmt.Errorf("%w", err)
	}

	defer uc.notify.Close(execution.ID().String())

	return CreateExecutionOutput{
		ExecutionID: execution.ID().String(),
		Status:      command.StatusQueued.String(),
	}, nil
}
