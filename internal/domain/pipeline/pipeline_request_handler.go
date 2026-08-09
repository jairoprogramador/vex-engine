package pipeline

import (
	"context"

	command "github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
)

// PipelineRequestHandler es el estado de la CADENA de pipeline, no el de la
// ejecución. `projectVersion` y `projectHeadHash` colgaban de aquí y se
// mudaron al agregado, que es de quien son (spec 07 §5.3): lo que queda es la
// lista de steps a recorrer, que sí muere con la cadena.
type PipelineRequestHandler struct {
	executionContext *command.ExecutionContext
	steps            []command.StepName

	// pipelineHeadHash es el commit del clon del pipelinecode. Vive AQUÍ y no en
	// el agregado —donde sí vive el del proyecto— porque su único consumidor está
	// en esta misma cadena: lo pone el clonador en la posición 02 y lo lee el
	// resolutor en la 09 (spec 18 §5.3). Es metadato del objeto, no un hecho de
	// la ejecución que alguien vaya a consultar después.
	pipelineHeadHash string

	// La identidad resuelta, anotada por el handler 09. La cadena de pipeline la
	// conoce a partir de ahí y muere con ella: quien la necesite fuera la lee del
	// registro, que es donde está escrita.
	content      deployment.Content
	deploymentID deployment.DeploymentID
	attempt      deployment.Attempt
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

func (r *PipelineRequestHandler) SetPipelineHeadHash(pipelineHeadHash string) {
	r.pipelineHeadHash = pipelineHeadHash
}

// PipelineHeadHash es el commit del clon del pipelinecode que se está usando.
func (r *PipelineRequestHandler) PipelineHeadHash() string {
	return r.pipelineHeadHash
}

func (r *PipelineRequestHandler) ProjectStatus() string {
	return r.executionContext.ProjectStatus()
}

func (r *PipelineRequestHandler) ExecutionID() string {
	return r.executionContext.ExecutionID().String()
}

// Runner es sobre qué corrió este intento, en la única forma que el motor
// conoce: la imagen del runtime declarada en el RequestInput. Vacío cuando no se
// declaró ninguna, que es el caso de un `vexd` invocado a mano.
func (r *PipelineRequestHandler) Runner() string {
	runtime := r.executionContext.Runtime()
	if runtime.IsEmpty() {
		return ""
	}
	return runtime.Image + ":" + runtime.Tag
}

// SetDeployment anota la identidad resuelta por el handler 09.
func (r *PipelineRequestHandler) SetDeployment(
	content deployment.Content, deploymentID deployment.DeploymentID, attempt deployment.Attempt) {

	r.content = content
	r.deploymentID = deploymentID
	r.attempt = attempt
}

func (r *PipelineRequestHandler) Content() deployment.Content { return r.content }

func (r *PipelineRequestHandler) DeploymentID() deployment.DeploymentID { return r.deploymentID }

func (r *PipelineRequestHandler) Attempt() deployment.Attempt { return r.attempt }

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
