package ports

import "context"

// ExecCommand representa el comando que el Executor lanzará como proceso hijo.
// Es distinto de vos.Command, que modela el comando tal como viene del pipelinecode
// (con nombre, outputs, templates, etc.). ExecCommand es la forma resuelta y lista
// para ejecutar en el runtime.
type ExecCommand struct {
	Line    string
	Workdir string
	Env     map[string]string
}

// RunResult es el resultado de ejecutar un ExecCommand en un runtime.
type RunResult struct {
	ExitCode int
	Output   string
}

// Executor define dónde corre el proceso hijo — no qué herramienta usa.
// Un mismo pipeline con docker build + kubectl apply + terraform puede ejecutarse
// en cualquier implementación de Executor sin cambiar los comandos.
type Executor interface {
	Run(ctx context.Context, cmd ExecCommand, env map[string]string) (RunResult, error)
}
