package dominio

import "context"

// Pipelines es lo que necesita Simulación de Definición de Pipeline: el pipeline comprobado, de una copia de
// trabajo o de un commit (IT-10 DEC-10.6). Sin DeHoy: SIM-1 no lo acepta.
type Pipelines interface {
	DeUnCommit(ctx context.Context, fuente, commit string) (Pipeline, error)
	DeUnaCopiaDeTrabajo(ctx context.Context, directorio string) (Pipeline, error)
	// VariablesEstandar son las variables que un pipeline puede usar sin declararlas; Definición solo sabe su
	// nombre, y quien aplica el pipeline les da valor.
	VariablesEstandar() []VariableEstandar
}

// Variables es lo que necesita Simulación de Resolución de Variables: interpolar en una petición sin intento
// (DEC-04.10), bajo un id de invocación efímero — el espacio temporal (simulacion.md, «Puertos»).
type Variables interface {
	// Declarar interpola y agrega los literales dados, en el ámbito de cada uno, y los deja visibles desde
	// ambito.
	Declarar(ctx context.Context, simulacion string, ambito Ambito, declaradas []VariableDeclarada) error
	// Interpolar sustituye cada ${var.<nombre>} por su valor entre lo visible desde ambito en esta
	// simulación. Falla si algún nombre usado no está disponible.
	Interpolar(ctx context.Context, simulacion string, ambito Ambito, texto string) (string, error)
	// RegistrarProducido pliega en memoria lo que un comando simulado dice que produciría.
	RegistrarProducido(ctx context.Context, simulacion, nombre, valor string, ambito Ambito) error
	// Cerrar libera la memoria de la simulación: una petición sin intento no deja rastro.
	Cerrar(ctx context.Context, simulacion string) error
}

// EspacioTemporal da un id de invocación nuevo por cada ambiente que se recorre — el «lugar donde interpolar
// el material, que desaparece al terminar» (simulacion.md, «Puertos»). Un id por ambiente, no uno por
// llamada a Simular: los literales del pipeline se repiten con distinto valor entre ambientes, y Resolución
// no distingue el ámbito al declarar por nombre (resolucion/dominio.VariablesDeUnaInvocacion.Declarar no
// sobreescribe) — compartir un id entre ambientes filtraría en silencio el valor del primero a los demás.
type EspacioTemporal interface {
	Nuevo() (string, error)
}
