package dto

// ExecutionInput contiene los parámetros de ejecución del pipeline.
type ExecutionInput struct {
	Step         string `json:"step"`
	Environment  string `json:"environment,omitempty"`
	RuntimeImage string `json:"runtime_image,omitempty"`
	RuntimeTag   string `json:"runtime_tag,omitempty"`
}
