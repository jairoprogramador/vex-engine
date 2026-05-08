package command

import (
	"context"
	"time"

	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

type ExecutionContext struct {
	ctx               *context.Context
	accumulatedVars   *ExecutionVariableMap
	execution         *Execution
	commandExecutable Executable
	stepExecutable    Executable
	emitter           domNotify.LogObserver
	statusEmitter     domNotify.StatusObserver
	workdir           string
	step              Step
	command           Command
	fileSessions      []FileInterpolatorSession
}

// NewExecutionContext compone el contexto compartido entre las tres cadenas
// (pipeline / step / command). El statusEmitter es opcional: si es nil, las
// llamadas a NotifyStage se descartan silenciosamente.
func NewExecutionContext(
	ctx *context.Context,
	execution *Execution,
	commandExecutable Executable,
	stepExecutable Executable,
	emitter domNotify.LogObserver,
	statusEmitter domNotify.StatusObserver) *ExecutionContext {

	return &ExecutionContext{
		ctx:               ctx,
		accumulatedVars:   NewExecutionVariableMap(),
		execution:         execution,
		emitter:           emitter,
		statusEmitter:     statusEmitter,
		commandExecutable: commandExecutable,
		stepExecutable:    stepExecutable,
		fileSessions:      make([]FileInterpolatorSession, 0),
	}
}

func (ec *ExecutionContext) StepExecutable() Executable {
	return ec.stepExecutable
}

func (ec *ExecutionContext) CommandExecutable() Executable {
	return ec.commandExecutable
}

func (ec *ExecutionContext) ProjectStatus() string {
	return ec.execution.ProjectStatus()
}

func (ec *ExecutionContext) SetWorkdir(workdir string) {
	ec.workdir = workdir
}

func (ec *ExecutionContext) Workdir() string {
	return ec.workdir
}

func (ec *ExecutionContext) SetProjectStatus(projectStatus string) {
	ec.execution.SetProjectStatus(projectStatus)
}

func (ec *ExecutionContext) SetCommand(command Command) {
	ec.command = command
}

func (ec *ExecutionContext) ProjectUrl() string {
	return ec.execution.ProjectUrl()
}

func (ec *ExecutionContext) ProjectRef() string {
	return ec.execution.ProjectRef()
}

func (ec *ExecutionContext) ProjectId() string {
	return ec.execution.ProjectId()
}

func (ec *ExecutionContext) ProjectName() string {
	return ec.execution.ProjectName()
}

func (ec *ExecutionContext) ProjectOrg() string {
	return ec.execution.ProjectOrg()
}

func (ec *ExecutionContext) ProjectTeam() string {
	return ec.execution.ProjectTeam()
}

func (ec *ExecutionContext) SetProjectLocalPath(projectLocalPath string) {
	ec.execution.SetProjectLocalPath(projectLocalPath)
}

func (ec *ExecutionContext) ProjectLocalPath() string {
	return ec.execution.ProjectLocalPath()
}

func (ec *ExecutionContext) PipelineUrl() string {
	return ec.execution.PipelineURL()
}

func (ec *ExecutionContext) PipelineRef() string {
	return ec.execution.PipelineRef()
}

func (ec *ExecutionContext) SetPipelineLocalPath(pipelineLocalPath string) {
	ec.execution.SetPipelineLocalPath(pipelineLocalPath)
}

func (ec *ExecutionContext) PipelineLocalPath() string {
	return ec.execution.PipelineLocalPath()
}

func (ec *ExecutionContext) Command() Command {
	return ec.command
}

func (ec *ExecutionContext) ResetFileSessions() {
	ec.fileSessions = make([]FileInterpolatorSession, 0)
}

func (ec *ExecutionContext) ExecutionID() ExecutionID {
	return ec.execution.ID()
}

func (ec *ExecutionContext) AccumulatedVars() *ExecutionVariableMap {
	return ec.accumulatedVars
}

func (ec *ExecutionContext) RemoveAccumulatedVar(name string) {
	ec.accumulatedVars.Remove(name)
}

func (ec *ExecutionContext) GetAccumulatedVar(name string) (Variable, bool) {
	return ec.accumulatedVars.Get(name)
}

func (ec *ExecutionContext) Environment() string {
	return ec.execution.Environment()
}

func (ec *ExecutionContext) SetEnvironment(environment string) {
	ec.execution.SetEnvironment(environment)
}

func (ec *ExecutionContext) StartedAt() time.Time {
	return ec.execution.StartedAt()
}
func (ec *ExecutionContext) StepFullName() string {
	return ec.step.FullName()
}

func (ec *ExecutionContext) Step() Step {
	return ec.step
}

func (ec *ExecutionContext) Emit(line string) {
	ec.emitter.Notify(ec.execution.ID().String(), line)
}

// NotifyStage reporta una transición de fase al StatusObserver registrado.
// Es seguro llamarlo aunque no haya statusEmitter (no-op).
func (ec *ExecutionContext) NotifyStage(stage string) {
	if ec.statusEmitter == nil {
		return
	}
	ec.statusEmitter.Notify(ec.execution.ID().String(), stage)
}

func (ec *ExecutionContext) Ctx() *context.Context {
	return ec.ctx
}

func (ec *ExecutionContext) AddFileSession(fileSession FileInterpolatorSession) {
	ec.fileSessions = append(ec.fileSessions, fileSession)
}

func (ec *ExecutionContext) RestoreFileSessions() error {
	for _, fileSession := range ec.fileSessions {
		if err := fileSession.Restore(); err != nil {
			return err
		}
	}
	return nil
}

func (ec *ExecutionContext) FilteredAccumulatedVars(filter func(Variable) bool) *ExecutionVariableMap {
	return ec.accumulatedVars.Filter(filter)
}

func (ec *ExecutionContext) AddAccumulatedVar(variable Variable) {
	ec.accumulatedVars.Add(variable)
}

func (ec *ExecutionContext) StepName() string {
	return ec.execution.Step()
}

func (ec *ExecutionContext) SetStepName(stepName StepName) {
	ec.step = NewStep(stepName)
	ec.execution.SetStep(stepName.Name())
}
