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

	// producedVars son las variables que el step EN CURSO extrajo del stdout de
	// sus comandos. Es el otro lado del reparto consume/produce de la spec 14: el
	// mapa acumulado dice qué ve un step, éste dice qué DEJÓ.
	//
	// Vive aquí, y no en el `StepRequestHandler`, porque quien lo alimenta es el
	// handler 05 de la cadena de COMANDO y quien lo lee es el `StepExecutable` de
	// la de step: cruza un borde de cadena, que es exactamente lo que este
	// contexto compartido existe para hacer. Es estado POR STEP, no de la
	// ejecución, y por eso se reinicia en el `before` del step junto a
	// `fileSessions`, que tiene la misma vida y el mismo motivo.
	producedVars *ExecutionVariableMap
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
		producedVars:      NewExecutionVariableMap(),
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

// SetProjectVersion / ProjectVersion / SetProjectHeadHash / ProjectHeadHash
// delegan en el agregado. Estos dos hechos vivían en PipelineRequestHandler
// (spec 07 §5.3); el contexto sólo los deja alcanzables desde las tres cadenas,
// como el resto de los datos de la ejecución.
func (ec *ExecutionContext) SetProjectVersion(projectVersion string) {
	ec.execution.SetProjectVersion(projectVersion)
}

func (ec *ExecutionContext) ProjectVersion() string {
	return ec.execution.ProjectVersion()
}

func (ec *ExecutionContext) SetProjectHeadHash(projectHeadHash string) {
	ec.execution.SetProjectHeadHash(projectHeadHash)
}

func (ec *ExecutionContext) ProjectHeadHash() string {
	return ec.execution.ProjectHeadHash()
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

// ResetProducedVars abre la cuenta del step que empieza. Sin este reinicio lo
// que produjo un step entraría en el registro del siguiente, que es justo el
// espacio plano que la spec 14 desmonta.
func (ec *ExecutionContext) ResetProducedVars() {
	ec.producedVars = NewExecutionVariableMap()
}

// ProducedVars es lo que el step en curso ha extraído hasta ahora.
func (ec *ExecutionContext) ProducedVars() *ExecutionVariableMap {
	return ec.producedVars
}

// AddProducedVar anota una variable extraída del stdout de un comando de este
// step. La llama el handler 05 de la cadena de comando, que es el único sitio
// del motor donde una variable NACE de la ejecución.
func (ec *ExecutionContext) AddProducedVar(variable Variable) {
	ec.producedVars.Add(variable)
}

func (ec *ExecutionContext) ExecutionID() ExecutionID {
	return ec.execution.ID()
}

// Runtime es la imagen sobre la que corre esta ejecución, tal como la declaró el
// RequestInput. Es CIRCUNSTANCIA —dónde corrió, no qué se pretendía hacer— y su
// consumidor es el hecho de apertura del intento (spec 18 §5.1).
func (ec *ExecutionContext) Runtime() ExecutionRuntime {
	return ec.execution.Runtime()
}

// Rollback es la ejecución pasada a la que ésta vuelve, o el valor cero cuando
// no vuelve a ninguna (spec 28 §5.5). Lo lee el handler 09, que es quien resuelve
// el ancla antes del primer step.
func (ec *ExecutionContext) Rollback() RollbackRequest {
	return ec.execution.Rollback()
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

// Now es el instante actual según el reloj inyectable del agregado (spec 07).
//
// Existe para las DURACIONES de los hechos (spec 19 §5.2), que es el campo con
// más valor del vocabulario y hoy no existe a ningún nivel. Sale del agregado y
// no de un `shared.Clock` inyectado en cada ejecutable por lo mismo que
// `StartedAt`: el reloj de la ejecución ya viaja con ella, y darle un segundo a
// los ejecutables permitiría que dos partes del mismo intento midieran contra
// relojes distintos.
func (ec *ExecutionContext) Now() time.Time {
	return ec.execution.Now()
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

// Aquí vivía `FilteredAccumulatedVars`, y su único llamador era el que
// componía el registro del step con el mapa acumulado entero. Muere con él
// (spec 14 §6): lo que se persiste ahora es `ProducedVars`, así que no queda
// ninguna razón para filtrar el acumulado desde fuera. El God Object se
// desmonta un campo por spec.

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
