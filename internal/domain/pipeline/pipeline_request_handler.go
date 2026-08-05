package pipeline

import (
	"context"

	command "github.com/jairoprogramador/vex-engine/internal/domain/command"
)

// PipelineRequestHandler es el estado de la CADENA de pipeline, no el de la
// ejecución. `projectVersion` y `projectHeadHash` colgaban de aquí y se
// mudaron al agregado, que es de quien son (spec 07 §5.3): lo que queda es la
// lista de steps a recorrer, que sí muere con la cadena.
type PipelineRequestHandler struct {
	executionContext *command.ExecutionContext
	steps            []command.StepName
}

func NewPipelineRequestHandler(executionContext *command.ExecutionContext) *PipelineRequestHandler {
	return &PipelineRequestHandler{
		executionContext: executionContext,
		steps:            make([]command.StepName, 0),
	}
}

func (r *PipelineRequestHandler) Execute() error {
	return r.executionContext.StepExecutable().Execute(r.executionContext)
}

func (rh *PipelineRequestHandler) SetWorkdir(workdir string) {
	rh.executionContext.SetWorkdir(workdir)
}

func (rh *PipelineRequestHandler) Ctx() *context.Context {
	return rh.executionContext.Ctx()
}

func (r *PipelineRequestHandler) ProjectId() string {
	return r.executionContext.ProjectId()
}

func (r *PipelineRequestHandler) ProjectName() string {
	return r.executionContext.ProjectName()
}

func (r *PipelineRequestHandler) SetProjectStatus(projectStatus string) {
	r.executionContext.SetProjectStatus(projectStatus)
}

func (r *PipelineRequestHandler) ProjectOrg() string {
	return r.executionContext.ProjectOrg()
}

func (r *PipelineRequestHandler) ProjectTeam() string {
	return r.executionContext.ProjectTeam()
}

func (r *PipelineRequestHandler) StepName() string {
	return string(r.executionContext.StepName())
}

func (r *PipelineRequestHandler) StepFullName() string {
	return r.executionContext.StepFullName()
}

func (r *PipelineRequestHandler) Environment() string {
	return r.executionContext.Environment()
}

func (r *PipelineRequestHandler) SetEnvironment(environment string) {
	r.executionContext.SetEnvironment(environment)
}

func (r *PipelineRequestHandler) ProjectUrl() string {
	return r.executionContext.ProjectUrl()
}

func (r *PipelineRequestHandler) ProjectRef() string {
	return r.executionContext.ProjectRef()
}

func (r *PipelineRequestHandler) SetProjectLocalPath(projectLocalPath string) {
	r.executionContext.SetProjectLocalPath(projectLocalPath)
}

func (r *PipelineRequestHandler) ProjectLocalPath() string {
	return r.executionContext.ProjectLocalPath()
}

func (r *PipelineRequestHandler) PipelineUrl() string {
	return r.executionContext.PipelineUrl()
}

func (r *PipelineRequestHandler) PipelineRef() string {
	return r.executionContext.PipelineRef()
}

func (r *PipelineRequestHandler) SetPipelineLocalPath(pipelineLocalPath string) {
	r.executionContext.SetPipelineLocalPath(pipelineLocalPath)
}

func (r *PipelineRequestHandler) PipelineLocalPath() string {
	return r.executionContext.PipelineLocalPath()
}

func (r *PipelineRequestHandler) Steps() []command.StepName {
	return r.steps
}

func (r *PipelineRequestHandler) SetStepName(stepName command.StepName) {
	r.executionContext.SetStepName(stepName)
}

func (r *PipelineRequestHandler) SetSteps(steps []command.StepName) {
	r.steps = steps
}

func (r *PipelineRequestHandler) SetProjectVersion(projectVersion string) {
	r.executionContext.SetProjectVersion(projectVersion)
}

func (r *PipelineRequestHandler) ProjectVersion() string {
	return r.executionContext.ProjectVersion()
}

func (r *PipelineRequestHandler) SetProjectHeadHash(projectHeadHash string) {
	r.executionContext.SetProjectHeadHash(projectHeadHash)
}

func (r *PipelineRequestHandler) ProjectHeadHash() string {
	return r.executionContext.ProjectHeadHash()
}

func (r *PipelineRequestHandler) Emit(line string) {
	r.executionContext.Emit(line)
}

func (r *PipelineRequestHandler) NotifyStage(stage string) {
	r.executionContext.NotifyStage(stage)
}

func (r *PipelineRequestHandler) AddAccumulatedVars(variable command.Variable) {
	r.executionContext.AccumulatedVars().Add(variable)
}
