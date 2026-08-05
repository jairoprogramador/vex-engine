package step

import (
	"context"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type StepRequestHandler struct {
	executionContext *command.ExecutionContext
	stepName         string
	stepStatus       command.StepStatus
	skipReason       SkipReason
}

func NewStepRequestHandler(executionContext *command.ExecutionContext, stepName string) *StepRequestHandler {

	requestHandler := &StepRequestHandler{
		stepName:         stepName,
		stepStatus:       command.StepFailure,
		skipReason:       SkipReasonNone,
		executionContext: executionContext,
	}
	return requestHandler
}

func (rh *StepRequestHandler) Execute() error {
	return rh.executionContext.CommandExecutable().Execute(rh.executionContext)
}

func (rh *StepRequestHandler) ProjectUrl() string {
	return rh.executionContext.ProjectUrl()
}

func (rh *StepRequestHandler) PipelineUrl() string {
	return rh.executionContext.PipelineUrl()
}

func (rh *StepRequestHandler) AddAccumulatedVars(variable command.Variable) {
	rh.executionContext.AccumulatedVars().Add(variable)
}

func (rh *StepRequestHandler) AddAccumulatedVarsAll(variables *command.ExecutionVariableMap) {
	for _, variable := range *variables {
		rh.executionContext.AccumulatedVars().Add(variable)
	}
}

func (rh *StepRequestHandler) AddCommand(command command.Command) {
	rh.executionContext.SetCommand(command)
}

func (rh *StepRequestHandler) AccumulatedVars() *command.ExecutionVariableMap {
	return rh.executionContext.AccumulatedVars()
}

func (rh *StepRequestHandler) StartedAt() time.Time {
	return rh.executionContext.StartedAt()
}

func (rh *StepRequestHandler) Environment() string {
	return rh.executionContext.Environment()
}

func (rh *StepRequestHandler) ProjectStatus() string {
	return rh.executionContext.ProjectStatus()
}

func (rh *StepRequestHandler) Ctx() *context.Context {
	return rh.executionContext.Ctx()
}

func (rh *StepRequestHandler) MarkStepSuccess() {
	rh.stepStatus = command.StepSuccess
}

// MarkStepSkipped registra que el step no se ejecutó, y por qué. Es un resultado
// propio: ni SUCCESS —que afirmaría una ejecución que no hubo— ni FAILURE.
func (rh *StepRequestHandler) MarkStepSkipped(reason SkipReason) {
	rh.stepStatus = command.StepSkipped
	rh.skipReason = reason
}

func (rh *StepRequestHandler) StepStatus() command.StepStatus {
	return rh.stepStatus
}

func (rh *StepRequestHandler) SkipReason() SkipReason {
	return rh.skipReason
}

func (rh *StepRequestHandler) WasSkipped() bool {
	return rh.stepStatus == command.StepSkipped
}

func (rh *StepRequestHandler) StepName() string {
	return rh.executionContext.StepName()
}

func (rh *StepRequestHandler) StepNameExe() string {
	return rh.executionContext.Step().Name()
}

func (rh *StepRequestHandler) StepFullName() string {
	return rh.executionContext.StepFullName()
}

func (rh *StepRequestHandler) SetStepName(stepName command.StepName) {
	rh.executionContext.SetStepName(stepName)
}

func (rh *StepRequestHandler) Emit(line string) {
	rh.executionContext.Emit(line)
}

func (rh *StepRequestHandler) PipelineLocalPath() string {
	return rh.executionContext.PipelineLocalPath()
}
