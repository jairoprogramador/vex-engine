package aggregates

import (
	"context"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
)

// Execution representa el ciclo de vida completo de una ejecución de pipeline.
// Es el agregado raíz del subdominio de ejecución HTTP-facing.
// cancelFn no se persiste — vive únicamente en memoria mientras el proceso corre.
type Execution struct {
	id          vos.ExecutionID
	status      vos.ExecutionStatus
	projectID   string
	projectName string
	pipelineURL string
	pipelineRef string
	step        string
	environment string
	runtimeCfg  vos.RuntimeConfig
	startedAt   time.Time
	finishedAt  *time.Time
	exitCode    *int
	cancelFn    context.CancelFunc
}

// NewExecution crea una nueva Execution con estado StatusQueued.
// startedAt se fija al momento de la creación.
func NewExecution(
	projectID, projectName string,
	pipelineURL, pipelineRef string,
	step, environment string,
	runtimeCfg vos.RuntimeConfig,
) *Execution {
	return &Execution{
		id:          vos.NewExecutionID(),
		status:      vos.StatusQueued,
		projectID:   projectID,
		projectName: projectName,
		pipelineURL: pipelineURL,
		pipelineRef: pipelineRef,
		step:        step,
		environment: environment,
		runtimeCfg:  runtimeCfg,
		startedAt:   time.Now(),
	}
}

// ID devuelve el identificador único de la ejecución.
func (e *Execution) ID() vos.ExecutionID {
	return e.id
}

// Status devuelve el estado actual del ciclo de vida.
func (e *Execution) Status() vos.ExecutionStatus {
	return e.status
}

// ProjectID devuelve el identificador del proyecto.
func (e *Execution) ProjectID() string {
	return e.projectID
}

// ProjectName devuelve el nombre legible del proyecto.
func (e *Execution) ProjectName() string {
	return e.projectName
}

// PipelineURL devuelve la URL del repositorio pipelinecode.
func (e *Execution) PipelineURL() string {
	return e.pipelineURL
}

// PipelineRef devuelve la referencia git (branch, tag o commit) del pipeline.
func (e *Execution) PipelineRef() string {
	return e.pipelineRef
}

// Step devuelve el nombre del step a ejecutar.
func (e *Execution) Step() string {
	return e.step
}

// Environment devuelve el ambiente de destino (sand, stag, prod…).
func (e *Execution) Environment() string {
	return e.environment
}

// RuntimeConfig devuelve la configuración de runtime (imagen/tag para executors de contenedor).
func (e *Execution) RuntimeConfig() vos.RuntimeConfig {
	return e.runtimeCfg
}

// StartedAt devuelve el instante en que se creó la ejecución.
func (e *Execution) StartedAt() time.Time {
	return e.startedAt
}

// FinishedAt devuelve el instante de finalización, o nil si aún no terminó.
func (e *Execution) FinishedAt() *time.Time {
	return e.finishedAt
}

// ExitCode devuelve el código de salida del proceso, o nil si aún no terminó.
func (e *Execution) ExitCode() *int {
	return e.exitCode
}

// MarkRunning transiciona el estado a StatusRunning.
func (e *Execution) MarkRunning() {
	e.status = vos.StatusRunning
}

// MarkSucceeded transiciona el estado a StatusSucceeded y registra finishedAt y exitCode.
func (e *Execution) MarkSucceeded(exitCode int) {
	now := time.Now()
	e.status = vos.StatusSucceeded
	e.finishedAt = &now
	e.exitCode = &exitCode
}

// MarkFailed transiciona el estado a StatusFailed y registra finishedAt y exitCode.
func (e *Execution) MarkFailed(exitCode int) {
	now := time.Now()
	e.status = vos.StatusFailed
	e.finishedAt = &now
	e.exitCode = &exitCode
}

// MarkCancelled transiciona el estado a StatusCancelled y registra finishedAt.
func (e *Execution) MarkCancelled() {
	now := time.Now()
	e.status = vos.StatusCancelled
	e.finishedAt = &now
}

// RehydrateExecution reconstruye un Execution desde storage.
// Solo para uso de implementaciones de ExecutionRepository.
func RehydrateExecution(
	id vos.ExecutionID,
	status vos.ExecutionStatus,
	projectID, projectName string,
	pipelineURL, pipelineRef string,
	step, environment string,
	runtimeCfg vos.RuntimeConfig,
	startedAt time.Time,
	finishedAt *time.Time,
	exitCode *int,
) *Execution {
	return &Execution{
		id:          id,
		status:      status,
		projectID:   projectID,
		projectName: projectName,
		pipelineURL: pipelineURL,
		pipelineRef: pipelineRef,
		step:        step,
		environment: environment,
		runtimeCfg:  runtimeCfg,
		startedAt:   startedAt,
		finishedAt:  finishedAt,
		exitCode:    exitCode,
	}
}

// SetCancelFn guarda la función de cancelación de contexto asociada a esta ejecución.
// Solo vive en memoria; no se persiste.
func (e *Execution) SetCancelFn(fn context.CancelFunc) {
	e.cancelFn = fn
}

// Cancel invoca cancelFn si fue registrada, interrumpiendo el contexto de ejecución.
func (e *Execution) Cancel() {
	if e.cancelFn != nil {
		e.cancelFn()
	}
}
