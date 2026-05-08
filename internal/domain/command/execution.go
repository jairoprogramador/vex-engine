package command

import (
	"context"
	"time"
)

type Execution struct {
	id          ExecutionID
	status      ExecutionStatus
	project     ExecutionProject
	pipeline    ExecutionPipeline
	step        string
	environment string
	runtime     ExecutionRuntime
	startedAt   time.Time
	finishedAt  *time.Time
	exitCode    *int
	cancelFn    context.CancelFunc
}

func NewExecution(project ExecutionProject, pipeline ExecutionPipeline,
	step, environment string,
	runtime ExecutionRuntime,
) *Execution {
	return &Execution{
		id:          NewExecutionID(),
		status:      StatusQueued,
		project:     project,
		pipeline:    pipeline,
		step:        step,
		environment: environment,
		runtime:     runtime,
		startedAt:   time.Now(),
	}
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

func (e *Execution) FinishedAt() *time.Time {
	return e.finishedAt
}

func (e *Execution) ExitCode() *int {
	return e.exitCode
}

func (e *Execution) MarkRunning() {
	e.status = StatusRunning
}

func (e *Execution) MarkSucceeded(exitCode int) {
	now := time.Now()
	e.status = StatusSucceeded
	e.finishedAt = &now
	e.exitCode = &exitCode
}

func (e *Execution) MarkFailed(exitCode int) {
	now := time.Now()
	e.status = StatusFailed
	e.finishedAt = &now
	e.exitCode = &exitCode
}

func (e *Execution) MarkCancelled() {
	now := time.Now()
	e.status = StatusCancelled
	e.finishedAt = &now
}

func RehydrateExecution(
	id ExecutionID,
	status ExecutionStatus,
	project ExecutionProject,
	pipeline ExecutionPipeline,
	step, environment string,
	runtime ExecutionRuntime,
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
		startedAt:   startedAt,
		finishedAt:  finishedAt,
		exitCode:    exitCode,
	}
}

func (e *Execution) SetCancelFn(fn context.CancelFunc) {
	e.cancelFn = fn
}

func (e *Execution) Cancel() {
	if e.cancelFn != nil {
		e.cancelFn()
	}
}
