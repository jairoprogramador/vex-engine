package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

// CreateExecutionOutput transporta el resultado de una ejecución.
//
// En el modo one-shot el use case bloquea hasta que la pipeline termina, así
// que el Status que sale de aquí es el TERMINAL y lo dice el agregado, no el
// que lo llama (spec 07 §5.2). Antes era siempre "queued", en el camino de
// éxito y en el de fallo, y quien reconstruía el estado real era la CLI a
// partir del error devuelto.
type CreateExecutionOutput struct {
	ExecutionID string
	Status      string
	ExitCode    *int
	StartedAt   time.Time
	FinishedAt  *time.Time
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
	clock              shared.Clock
	notify             domNotify.LogObserver
	status             domNotify.StatusObserver
}

// NewCreateExecutionUseCase compone el use case sin observers; el caller debe
// usar WithObservers antes de Execute si quiere recibir logs/stages.
func NewCreateExecutionUseCase(
	executablePipeline command.Executable,
	executableCommand command.Executable,
	executableStep command.Executable,
	clock shared.Clock) *CreateExecutionUseCase {
	return &CreateExecutionUseCase{
		executablePipeline: executablePipeline,
		executableCommand:  executableCommand,
		executableStep:     executableStep,
		clock:              clock,
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

// Execute valida el comando, lanza la ejecución de la pipeline y CIERRA el
// ciclo de vida del agregado: es la única capa que ve el éxito y el fallo, así
// que es la que los registra (spec 07 §5.2, «un solo dueño por hecho»).
// Retorna el estado terminal tal como lo publica el agregado.
//
// notify debe ser no-nil (el caller usualmente provee un MultiObserver vacío);
// status puede ser nil — los handlers usan ExecutionContext.NotifyStage que
// tolera nil receiver.
func (uc *CreateExecutionUseCase) Execute(ctx context.Context, request dto.RequestInput, executionIdInput string) (CreateExecutionOutput, error) {
	if uc.notify == nil {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: notify observer is required (use WithObservers)")
	}
	if request.Execution.Step == "" {
		return CreateExecutionOutput{}, fmt.Errorf("use case create execution: step is required")
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

	executionId := command.NewExecutionID(executionIdInput)

	execution := command.NewExecution(
		executionId,
		command.NewExecutionProject(request.Project.Id, request.Project.Name, projectUrl.String(), request.Project.Ref, request.Project.Org, request.Project.Team),
		command.NewExecutionPipeline(pipelineUrl.String(), request.Pipeline.Ref),
		request.Execution.Step,
		request.Execution.Environment,
		command.NewExecutionRuntime(request.Execution.RuntimeImage, request.Execution.RuntimeTag),
		uc.clock,
	)

	childCtx, cancelFn := context.WithCancel(ctx)
	execution.SetCancelFn(cancelFn)
	// El `defer` es la corrección de la fuga: el cancelFn se guardaba y no se
	// llamaba nunca, así que el contexto hijo sobrevivía al proceso que lo creó.
	defer execution.Cancel()

	executionContext := command.NewExecutionContext(
		&childCtx,
		execution,
		uc.executableCommand,
		uc.executableStep,
		uc.notify,
		uc.status,
	)

	if err := execution.MarkRunning(); err != nil {
		return uc.output(execution), fmt.Errorf("use case create execution: %w", err)
	}

	runErr := uc.executablePipeline.Execute(executionContext)
	uc.markTerminal(ctx, execution, runErr)

	if runErr != nil {
		return uc.output(execution), fmt.Errorf("%w", runErr)
	}
	return uc.output(execution), nil
}

// markTerminal pliega el resultado observado al estado del agregado.
//
// Una cancelación no es un fallo aunque llegue disfrazada de uno: al cancelar
// el contexto, el comando en curso muere y la cadena devuelve error. Lo que
// distingue los dos casos es el contexto padre, y por eso se mira ÉL y no el
// error. Es la diferencia entre `cancelled` —una decisión— e `interrupted`
// —una desgracia— que el vocabulario de la spec 17 necesita poder expresar.
//
// El error de la transición se descarta a propósito: sólo puede fallar si el
// agregado ya está en un estado terminal, y eso significa que alguien más
// —la cancelación— ya registró un hecho más específico que éste.
func (uc *CreateExecutionUseCase) markTerminal(ctx context.Context, execution *command.Execution, runErr error) {
	if errors.Is(ctx.Err(), context.Canceled) {
		_ = execution.MarkCancelled()
		return
	}
	if runErr != nil {
		_ = execution.MarkFailed(exitCodeOf(runErr))
		return
	}
	_ = execution.MarkSucceeded(0)
}

// exitCodeOf recupera el exit code del comando que falló. Cualquier otro fallo
// —un clone, una validación del pipelinecode— no tiene exit code propio y se
// registra con el genérico.
func exitCodeOf(err error) int {
	var commandFailed *command.CommandFailedError
	if errors.As(err, &commandFailed) {
		return commandFailed.ExitCode()
	}
	return command.DefaultFailureExitCode
}

func (uc *CreateExecutionUseCase) output(execution *command.Execution) CreateExecutionOutput {
	return CreateExecutionOutput{
		ExecutionID: execution.ID().String(),
		Status:      execution.Status().String(),
		ExitCode:    execution.ExitCode(),
		StartedAt:   execution.StartedAt(),
		FinishedAt:  execution.FinishedAt(),
	}
}
