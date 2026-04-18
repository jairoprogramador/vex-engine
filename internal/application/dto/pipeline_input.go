package dto

// PipelineInput identifica el repositorio pipelinecode a ejecutar.
type PipelineInput struct {
	URL string
	Ref string
}
