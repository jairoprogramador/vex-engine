package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
	domSync "github.com/jairoprogramador/vex-engine/internal/domain/sync"
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
	emitter            *record.Emitter
	synchronizer       *domSync.Synchronizer
	notify             domNotify.LogObserver
	status             domNotify.StatusObserver
}

// NewCreateExecutionUseCase compone el use case sin observers; el caller debe
// usar WithObservers antes de Execute si quiere recibir logs/stages.
//
// El emisor entra por el constructor y no por `WithObservers` porque no depende
// de flags: registrar es incondicional (spec 19 §5.6), y los observers no. El
// sincronizador entra por lo mismo: empujar tampoco depende de flags, depende de
// la configuración de destino, que se resuelve al cablear (spec 21).
func NewCreateExecutionUseCase(
	executablePipeline command.Executable,
	executableCommand command.Executable,
	executableStep command.Executable,
	clock shared.Clock,
	emitter *record.Emitter,
	synchronizer *domSync.Synchronizer) *CreateExecutionUseCase {
	return &CreateExecutionUseCase{
		executablePipeline: executablePipeline,
		executableCommand:  executableCommand,
		executableStep:     executableStep,
		clock:              clock,
		emitter:            emitter,
		synchronizer:       synchronizer,
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

	// El destino del rollback viaja SIN interpretar (spec 28 §5.5): quien lo
	// compone como value object es el handler 09, y quien lo rechaza por
	// malformado es el borde. Aquí sólo se transporta, que es lo único que esta
	// capa puede hacer con él sin importar `deployment` desde `command`.
	if request.RollbackTo != nil {
		execution.SetRollback(command.RollbackRequest{
			DeploymentID: request.RollbackTo.DeploymentID,
			Attempt:      request.RollbackTo.Attempt,
		})
	}

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

	// La redacción es política de la FRONTERA (spec 20 §5.1'), pero el
	// vocabulario con el que redacta es del dominio: son los valores que el
	// proceso conoce, y el único que los tiene todos es el mapa acumulado. Aquí
	// es donde los dos se encuentran, porque es la capa que crea el contexto y la
	// que recibe los observadores.
	//
	// Es OPCIONAL a propósito y no una segunda obligación del puerto: un
	// observador que no redacta —los dos concretos, y el mudo de los tests— sigue
	// siendo un `LogObserver` y nada más.
	if aware, ok := uc.notify.(domNotify.VocabularyAware); ok {
		aware.UseVocabulary(executionContext.AccumulatedVars())
	}

	if err := execution.MarkRunning(); err != nil {
		return uc.output(execution), fmt.Errorf("use case create execution: %w", err)
	}

	runErr := uc.executablePipeline.Execute(executionContext)
	uc.markTerminal(ctx, execution, runErr)

	// El desenlace se registra AQUÍ y sólo aquí, por lo mismo que el ciclo de vida
	// del agregado se cierra aquí (spec 07 §5.2): es la única capa que ve tanto el
	// éxito como el fallo, y la única que sabe distinguir una CANCELACIÓN de un
	// fallo cualquiera — a la cadena la cancelación le llega como un error más.
	//
	// Y su AUSENCIA es lo que justifica el modelo entero: un intento sin este
	// hecho pliega a `interrupted`, con el último step alcanzado, en vez de a
	// nada. Por eso no se emite nunca `interrupted`: quien está vivo para
	// escribirlo, por definición, no fue interrumpido.
	if err := uc.recordAttemptFinished(ctx, execution); err != nil {
		runErr = joinRunError(runErr, err)
	}

	// Y el empuje de cierre, con la tira ya cerrada: es el gancho natural, porque
	// `attempt_finished` es el último hecho y esta capa es la única que lo ve
	// (spec 21 §5.3). Es el MISMO código que el empuje por step —lo pendiente
	// desde el `ack`— y no una vía final aparte; no tener dos es lo que hace que
	// la recuperación de un empuje fallido salga gratis.
	//
	// El contexto es el del PROCESO y no el hijo cancelable, por lo mismo que en
	// `recordAttemptFinished`: el hijo está muerto justo en el caso que más
	// importa empujar, y una cancelación registrada que no llega al destino es
	// indistinguible de una máquina que desapareció.
	//
	// Consecuencia declarada de colgarlo de aquí: un intento INTERRUMPIDO —muerte
	// dura, OOM— no llega a empujarse nunca por esta vía. Lo acotado es lo que el
	// empuje por step ya dejó puesto, que es exactamente lo que §5.6 dice que esta
	// spec no garantiza.
	syncCtx := ctx
	uc.synchronizer.Push(&syncCtx)

	if runErr != nil {
		return uc.output(execution), fmt.Errorf("%w", runErr)
	}
	return uc.output(execution), nil
}

// recordAttemptFinished emite el cierre del intento con el estado que el
// AGREGADO publica, no con uno deducido del error.
//
// Si el intento nunca llegó a tener identidad —la ejecución falló antes del
// resolutor de despliegue— el emisor retiene el hecho y se pierde con el
// proceso. Es correcto y no un agujero: sin `deployment_id` no hay tira a la que
// pertenezca, y escribirlo en otro sitio sería inventar una segunda dirección
// para los mismos hechos (spec 18 §5.1).
func (uc *CreateExecutionUseCase) recordAttemptFinished(
	ctx context.Context, execution *command.Execution) error {

	status, ok := attemptStatusOf(execution.Status())
	if !ok {
		// El agregado no alcanzó un estado terminal. No hay desenlace que declarar,
		// y `Fold` ya sabe leer esa ausencia.
		return nil
	}

	// El contexto que se pasa es el del PROCESO y no el hijo cancelable: el hijo
	// está muerto justo en el caso que más importa registrar, y escribir el
	// desenlace de una cancelación es la única forma de distinguirla de una
	// interrupción.
	terminalCtx := ctx
	if err := uc.emitter.Emit(&terminalCtx, record.AttemptFinished{Status: status}); err != nil {
		return fmt.Errorf("registrar el desenlace del intento: %w", err)
	}
	return nil
}

// attemptStatusOf traduce el estado terminal del agregado al desenlace del
// registro.
//
// `interrupted` no aparece porque no se puede alcanzar: es lo que se DERIVA de
// la ausencia de este hecho, y `AttemptFinished.Validate` lo rechaza
// explícitamente.
func attemptStatusOf(status command.ExecutionStatus) (record.AttemptStatus, bool) {
	switch status {
	case command.StatusSucceeded:
		return record.AttemptSucceeded, true
	case command.StatusFailed:
		return record.AttemptFailed, true
	case command.StatusCancelled:
		return record.AttemptCancelled, true
	default:
		return "", false
	}
}

// joinRunError compone el fallo de la ejecución con el de su registro, con la
// misma regla que el Template Method: la causa antes que la consecuencia.
//
// Que un fallo al registrar tumbe una ejecución que fue bien es deliberado y es
// la otra cara de §5.6: el registro es la fuente de la que se deriva todo lo
// demás, así que «no pude escribir el desenlace» no es un detalle cosmético. Es
// lo contrario del criterio del ALMACÉN de estado, donde no poder guardar no
// invalida el despliegue que sí ocurrió — allí lo que se pierde es la pista de
// un recurso, aquí lo que se pierde es la historia entera del intento.
func joinRunError(runErr, recordErr error) error {
	if runErr == nil {
		return recordErr
	}
	return errors.Join(runErr, recordErr)
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
