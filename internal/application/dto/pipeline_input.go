package dto

// PipelineInput identifica el repositorio pipelinecode a ejecutar.
type PipelineInput struct {
	Url string `json:"url"`
	Ref string `json:"ref"`
}
