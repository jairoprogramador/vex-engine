package usecase

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

// CreateExecutionOutput transporta el resultado inmediato de una ejecución.
// En el modo one-shot (M3+) el use case bloquea hasta que la pipeline termina,
// por lo que el Status reflejado aquí es el inicial ("queued"); el status terminal
// se reporta vía el StatusObserver/StatusReporter.
type CreateExecutionOutput struct {
	ExecutionID string
	Status      string
}

// CreateExecutionUseCase valida el request mínimo y ejecuta la pipeline de forma
// sincrónica. Acepta un LogObserver y un StatusObserver vía interfaces de dominio
// para mantener la regla de dependencia (la capa application no conoce concreciones
// como SupabaseLogObserver o MultiObserver).
//
// Los observers son opcionales en el constructor: el caller puede registrarlos
// per-ejecución vía WithObservers para que el wiring de RunCommand pueda
// construirlos sólo cuando los flags relevantes están presentes.
type CreateExecutionUseCase struct {
	executablePipeline command.Executable
	executableCommand  command.Executable
	executableStep     command.Executable
	notify             domNotify.LogObserver
	status             domNotify.StatusObserver
}

// NewCreateExecutionUseCase compone el use case sin observers; el caller debe
// usar WithObservers antes de Execute si quiere recibir logs/stages.
func NewCreateExecutionUseCase(
	executablePipeline command.Executable,
	executableCommand command.Executable,
	executableStep command.Executable) *CreateExecutionUseCase {
	return &CreateExecutionUseCase{
		executablePipeline: executablePipeline,
		executableCommand:  executableCommand,
		executableStep:     executableStep,
	}
}

// WithObservers retorna una copia del use case con observers inyectados, sin
// mutar el original. Permite que el factory construya la instancia una vez y
// que cada `vexd run` añada sus observers (que sí dependen de flags) sin
// reconstruir toda la cadena de pipeline.
func (uc *CreateExecutionUseCase) WithObservers(notify domNotify.LogObserver, status domNotify.StatusObserver) *CreateExecutionUseCase {
	clone := *uc
	clone.notify = notify
	clone.status = status
	return &clone
}

// Execute valida el comando y lanza la ejecución de la pipeline.
// Retorna ExecutionID y Status="queued" tras lanzar la cadena.
// El consumidor (RunCommand) interpreta el error retornado para decidir el
// status terminal (succeeded / failed) que reportará al StatusReporter.
//
// notify debe ser no-nil (el caller usualmente provee un MultiObserver vacío);
// status puede ser nil — los handlers usan ExecutionContext.NotifyStage que
// tolera nil receiver.
func (uc *CreateExecutionUseCase) Execute(ctx context.Context, request dto.RequestInput) (CreateExecutionOutput, error) {
	if uc.notify == nil {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: notify observer is required (use WithObservers)")
	}
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

	childCtx, cancelFn := context.WithCancel(ctx)
	execution.SetCancelFn(cancelFn)

	executionContext := command.NewExecutionContext(
		&childCtx,
		execution,
		uc.executableCommand,
		uc.executableStep,
		uc.notify,
		uc.status,
	)

	if err := uc.executablePipeline.Execute(executionContext); err != nil {
		return CreateExecutionOutput{
			ExecutionID: execution.ID().String(),
			Status:      command.StatusQueued.String(),
		}, fmt.Errorf("%w", err)
	}

	return CreateExecutionOutput{
		ExecutionID: execution.ID().String(),
		Status:      command.StatusQueued.String(),
	}, nil
}
