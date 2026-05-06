package notify

// StatusObserver recibe transiciones de fase ("stage") durante una ejecución.
// Es análogo a LogObserver pero la granularidad es mucho más gruesa: cada handler
// de la cadena de pipeline emite un único stage al iniciar su trabajo principal.
//
// Stages canónicos: "initializing", "cloning_project", "cloning_pipeline",
// "loading_environment", "loading_steps", "calculating_version",
// "computing_fingerprint", "running_step:<step_name>".
type StatusObserver interface {
	Notify(executionID string, stage string)
}
