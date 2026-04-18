package dto

// ExecutionInput contiene los parámetros de ejecución del pipeline.
type ExecutionInput struct {
	Step          string
	Environment   string
	ParametersURL string
	SecretsURL    string
	RuntimeImage  string
	RuntimeTag    string
}
