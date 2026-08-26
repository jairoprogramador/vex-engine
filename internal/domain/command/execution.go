package command

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

// ErrTransicionIlegal es el error de toda transición que el ciclo de vida no
// admite: marcar dos veces un estado terminal, o marcar uno sin haber
// empezado. No es un caso excepcional a evitar — es el mecanismo por el que
// una cancelación gana sobre el fallo que ella misma provocó (spec 07 §5.4).
var ErrTransicionIlegal = errors.New("transición de estado ilegal")

type Execution struct {
	id          ExecutionID
	project     ExecutionProject
	pipeline    ExecutionPipeline
	step        string
	environment string
	runtime     ExecutionRuntime
	clock       shared.Clock

	// rollback es la ejecución pasada a la que esta vuelve, si vuelve a alguna
	// (spec 28). Es ENTRADA de la ejecución, como el step o el ambiente, y por eso
	// vive en el agregado y no en el estado de una cadena: el handler 09 la lee
	// para resolver el ancla, y el hecho de apertura la publica.
	rollback RollbackRequest

	// projectVersion y projectHeadHash son hechos DE LA EJECUCIÓN: qué versión
	// se calculó y sobre qué commit. Vivían en PipelineRequestHandler —el
	// estado de una cadena— y eso los hacía morir con ella (spec 07 §5.3).
	projectVersion  string
	projectHeadHash string

	// mu protege el ciclo de vida. La cancelación llega desde fuera del hilo
	// que ejecuta la pipeline, así que status/finishedAt/exitCode son los
	// únicos campos del agregado con dos escritores posibles.
	mu         sync.Mutex
	status     ExecutionStatus
	startedAt  time.Time
	finishedAt *time.Time
	exitCode   *int

	cancelFn context.CancelFunc
}

func NewExecution(executionId ExecutionID, project ExecutionProject, pipeline ExecutionPipeline,
	step, environment string,
	runtime ExecutionRuntime,
	clock shared.Clock,
) *Execution {
	return &Execution{
		id:          executionId,
		status:      StatusQueued,
		project:     project,
		pipeline:    pipeline,
		step:        step,
		environment: environment,
		runtime:     runtime,
		clock:       clock,
		startedAt:   clock.Now(),
	}
}

// SetRollback anota la ejecución pasada a la que ésta vuelve.
//
// Es un setter y no un parámetro más del constructor —que ya lleva siete— por lo
// mismo que `SetProjectStatus`: el valor cero es el caso normal y significa lo
// correcto, «esto no es un rollback». Un campo opcional que hay que pasar
// siempre convierte a todos los llamadores en testigos de una decisión que no
// tomaron.
func (e *Execution) SetRollback(rollback RollbackRequest) {
	e.rollback = rollback
}

// Rollback es la ejecución pasada a la que ésta vuelve, o el valor cero.
func (e *Execution) Rollback() RollbackRequest {
	return e.rollback
}

func (e *Execution) ProjectStatus() string {
	return e.project.ProjectStatus()
}

func (e *Execution) SetProjectStatus(projectStatus string) {
	e.project.SetProjectStatus(projectStatus)
}

func (e *Execution) ID() ExecutionID {
	return e.id
}

func (e *Execution) Status() ExecutionStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.status
}

func (e *Execution) ProjectId() string {
	return e.project.ProjectId()
}

func (e *Execution) ProjectName() string {
	return e.project.ProjectName()
}

func (e *Execution) ProjectOrg() string {
	return e.project.ProjectOrg()
}

func (e *Execution) ProjectTeam() string {
	return e.project.ProjectTeam()
}

func (e *Execution) ProjectUrl() string {
	return e.project.ProjectUrl()
}

func (e *Execution) ProjectRef() string {
	return e.project.ProjectRef()
}

func (e *Execution) SetProjectLocalPath(projectLocalPath string) {
	e.project.SetProjectLocalPath(projectLocalPath)
}

func (e *Execution) ProjectLocalPath() string {
	return e.project.ProjectLocalPath()
}

func (e *Execution) SetProjectVersion(projectVersion string) {
	e.projectVersion = projectVersion
}

func (e *Execution) ProjectVersion() string {
	return e.projectVersion
}

func (e *Execution) SetProjectHeadHash(projectHeadHash string) {
	e.projectHeadHash = projectHeadHash
}

func (e *Execution) ProjectHeadHash() string {
	return e.projectHeadHash
}

func (e *Execution) PipelineURL() string {
	return e.pipeline.PipelineUrl()
}

func (e *Execution) PipelineRef() string {
	return e.pipeline.PipelineRef()
}

func (e *Execution) SetPipelineLocalPath(pipelineLocalPath string) {
	e.pipeline.SetPipelineLocalPath(pipelineLocalPath)
}

func (e *Execution) PipelineLocalPath() string {
	return e.pipeline.PipelineLocalPath()
}

func (e *Execution) Step() string {
	return e.step
}

// SetStep actualiza el paso lógico en curso (p. ej. al avanzar pasos del pipeline).
func (e *Execution) SetStep(step string) {
	e.step = step
}

func (e *Execution) Environment() string {
	return e.environment
}

func (e *Execution) SetEnvironment(environment string) {
	e.environment = environment
}

func (e *Execution) Runtime() ExecutionRuntime {
	return e.runtime
}

func (e *Execution) StartedAt() time.Time {
	return e.startedAt
}

// Now es el instante actual según el reloj de ESTA ejecución.
//
// El reloj ya vivía aquí —de él salen `startedAt` y `finishedAt`— y lo que la
// spec 19 necesita es medir duraciones dentro de la ejecución. Publicarlo por
// aquí y no inyectar un `shared.Clock` en cada ejecutable es lo que impide que
// dos partes del mismo intento midan contra relojes distintos, y mantiene la
// prohibición de `time.Now()` en el dominio (spec 07) en un solo sitio.
func (e *Execution) Now() time.Time {
	return e.clock.Now()
}

func (e *Execution) FinishedAt() *time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.finishedAt
}

func (e *Execution) ExitCode() *int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.exitCode
}

// Duration es el tiempo que la ejecución estuvo viva. Sólo existe una vez
// alcanzado el estado terminal: mientras corre, no hay duración que medir.
func (e *Execution) Duration() (time.Duration, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.finishedAt == nil {
		return 0, false
	}
	return e.finishedAt.Sub(e.startedAt), true
}

// MarkRunning abre el trabajo. Sólo se puede empezar lo que está encolado.
func (e *Execution) MarkRunning() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.status != StatusQueued {
		return fmt.Errorf("%w: %s → %s", ErrTransicionIlegal, e.status, StatusRunning)
	}
	e.status = StatusRunning
	return nil
}

// MarkSucceeded cierra el trabajo con éxito. Sólo se puede terminar lo que
// está corriendo: un estado terminal no se pisa (§5.3').
func (e *Execution) MarkSucceeded(exitCode int) error {
	return e.finish(StatusSucceeded, &exitCode)
}

// MarkFailed cierra el trabajo con fallo, con el exit code del comando que lo
// provocó.
func (e *Execution) MarkFailed(exitCode int) error {
	return e.finish(StatusFailed, &exitCode)
}

// MarkCancelled cierra el trabajo por decisión de quien lo lanzó. No lleva
// exit code: no hay comando cuyo resultado reportar, y ésa es exactamente la
// diferencia con un fallo. Se admite desde `queued` porque la señal puede
// llegar antes de que la cadena arranque.
func (e *Execution) MarkCancelled() error {
	return e.finish(StatusCancelled, nil)
}

func (e *Execution) finish(status ExecutionStatus, exitCode *int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.status.IsTerminal() {
		return fmt.Errorf("%w: %s → %s", ErrTransicionIlegal, e.status, status)
	}
	if e.status == StatusQueued && status != StatusCancelled {
		return fmt.Errorf("%w: %s → %s", ErrTransicionIlegal, e.status, status)
	}

	now := e.clock.Now()
	e.status = status
	e.finishedAt = &now
	e.exitCode = exitCode
	return nil
}

func RehydrateExecution(
	id ExecutionID,
	status ExecutionStatus,
	project ExecutionProject,
	pipeline ExecutionPipeline,
	step, environment string,
	runtime ExecutionRuntime,
	clock shared.Clock,
	startedAt time.Time,
	finishedAt *time.Time,
	exitCode *int,
) *Execution {
	return &Execution{
		id:          id,
		status:      status,
		project:     project,
		pipeline:    pipeline,
		step:        step,
		environment: environment,
		runtime:     runtime,
		clock:       clock,
		startedAt:   startedAt,
		finishedAt:  finishedAt,
		exitCode:    exitCode,
	}
}

func (e *Execution) SetCancelFn(fn context.CancelFunc) {
	e.cancelFn = fn
}

// Cancel libera el contexto de la ejecución. Se invoca siempre —con `defer`
// desde el use case— porque un `context.WithCancel` cuyo cancel no se llama
// filtra la goroutine que lo vigila.
func (e *Execution) Cancel() {
	if e.cancelFn != nil {
		e.cancelFn()
	}
}
