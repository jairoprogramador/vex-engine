package dto

// RequestInput es el DTO que transporta todos los parámetros necesarios
// para iniciar una ejecución de pipeline. Lo envía el HTTP handler al orchestrator.
type RequestInput struct {
	Project   ProjectInput
	Pipeline  PipelineInput
	Execution ExecutionInput
}
