package dto

// RequestInput es el DTO que transporta todos los parámetros necesarios para
// iniciar una ejecución de pipeline. Lo construye el cliente (vex CLI o la edge
// function trigger-deploy) y lo entrega a `vexd run` como JSON vía:
//   - flag --input <path>
//   - env var VEX_REQUEST_INPUT (base64 + JSON)
//   - stdin
//
// SchemaVersion identifica la versión del contrato. Es obligatorio y debe ser
// igual a la única versión soportada por este binario. Cualquier otro valor
// (incluido 0) hace que el input se rechace como "input invalido".
type RequestInput struct {
	SchemaVersion int            `json:"schema_version"`
	Project       ProjectInput   `json:"project"`
	Pipeline      PipelineInput  `json:"pipeline"`
	Execution     ExecutionInput `json:"execution"`
}
