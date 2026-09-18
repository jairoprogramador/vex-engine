package publicado

import "context"

// Lo que Definición de Pipeline publica, por cliente (modelo/contextos/definicion.md, «Lo que le piden»). El
// pipeline se trae de una fuente, el de hoy o el de un commit, y llega comprobado: si no pasa la comprobación,
// se recibe *FallosDeComprobacion. Si la fuente o el commit no están, ErrNoExiste, y si no se pueden pedir,
// ErrInvalido.
//
// VariablesEstandar son las variables que un pipeline puede usar sin declararlas. Definición solo sabe su
// nombre: quien aplica el pipeline es quien les da valor, y se asegura de que están todas.

// ParaEjecucion es lo que usa Ejecución de Pipeline: el pipeline de hoy, o el de un commit para un rollback
// (IT-03 DEC-03.9).
type ParaEjecucion interface {
	DeHoy(ctx context.Context, fuente string) (Pipeline, error)
	DeUnCommit(ctx context.Context, fuente, commit string) (Pipeline, error)
	VariablesEstandar() []VariableEstandar
}

// ParaResolucion es lo que usa Resolución de Variables. Las variables declaradas y las formas de ámbito y de
// variable de salida van en el pipeline, que se pide por su commit para leer el mismo que el intento.
type ParaResolucion interface {
	DeUnCommit(ctx context.Context, fuente, commit string) (Pipeline, error)
	VariablesEstandar() []VariableEstandar
}

// ParaSimulacion es lo que usa Simulación de Pipeline: el pipeline entero, de una copia de trabajo o de un
// commit (IT-10 DEC-10.6), o sus fallos.
type ParaSimulacion interface {
	DeUnCommit(ctx context.Context, fuente, commit string) (Pipeline, error)
	DeUnaCopiaDeTrabajo(ctx context.Context, directorio string) (Pipeline, error)
	VariablesEstandar() []VariableEstandar
}
