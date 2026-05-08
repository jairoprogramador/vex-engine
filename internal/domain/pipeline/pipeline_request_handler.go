package pipeline

import (
	"context"
	"time"

	command "github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type PipelineRequestHandler struct {
	executionContext *command.ExecutionContext
	steps            []command.StepName
	projectVersion   string
	projectHeadHash  string
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

func (r *PipelineRequestHandler) startedAt() time.Time {
	return r.executionContext.StartedAt()
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
	r.projectVersion = projectVersion
}

func (r *PipelineRequestHandler) ProjectVersion() string {
	return r.projectVersion
}

func (r *PipelineRequestHandler) SetProjectHeadHash(projectHeadHash string) {
	r.projectHeadHash = projectHeadHash
}

func (r *PipelineRequestHandler) ProjectHeadHash() string {
	return r.projectHeadHash
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
