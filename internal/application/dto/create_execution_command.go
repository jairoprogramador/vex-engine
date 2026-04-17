package dto

// CreateExecutionCommand es el DTO que transporta todos los parámetros necesarios
// para iniciar una ejecución de pipeline. Lo envía el HTTP handler al orchestrator.
type CreateExecutionCommand struct {
	Project   ProjectInput
	Pipeline  PipelineInput
	Execution ExecutionInput
}

// ProjectInput contiene los datos de identificación del proyecto.
// Si URL está vacío, el orchestrator trata ID como una ruta local (modo legacy CLI).
type ProjectInput struct {
	ID   string
	Name string
	Team string
	Org  string
	URL  string
	Ref  string
}

// PipelineInput identifica el repositorio pipelinecode a ejecutar.
type PipelineInput struct {
	URL string
	Ref string
}

// ExecutionInput contiene los parámetros de ejecución del pipeline.
type ExecutionInput struct {
	Step         string
	Environment  string
	RuntimeImage string
	RuntimeTag   string
}
