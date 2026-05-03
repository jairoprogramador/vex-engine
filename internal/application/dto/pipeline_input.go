package dto

// PipelineInput identifica el repositorio pipelinecode a ejecutar.
type PipelineInput struct {
	Url string
	Ref string
}
