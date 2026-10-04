package publicado

import "context"

// Lo que Resolución de Variables publica, por cliente (modelo/contextos/resolucion.md, «Lo que publica»).
// Ninguna variable lleva su valor: sale solo por interpolación (el texto ya resuelto) o hacia la relación
// reservada de Historial, nunca hacia un tipo de este paquete.

// ParaEjecucion es lo que usa Ejecución de Pipeline en un intento real.
type ParaEjecucion interface {
	// VariablesDeUnPaso declara las variables estándar (con los valores que da estandar) y los literales del
	// pipeline de ese commit que se ven desde ambito, y devuelve lo que queda visible. estandar es un puente
	// mientras RD-06 no exista para darlos de otra forma.
	VariablesDeUnPaso(
		ctx context.Context, intento, paso string, ambito Ambito, fuente, commit string, estandar map[string]string,
	) ([]Variable, error)
	// Interpolar sustituye cada ${var.<nombre>} por su valor entre lo visible desde ambito en este intento.
	Interpolar(ctx context.Context, intento, paso string, ambito Ambito, texto string) (string, error)
	// HashDeLasVariables resume lo que un paso consume, visible desde ambito: las del pipeline, las que
	// produjeron pasos anteriores y los metadatos — no las generadas por el motor — que referencian los textos
	// del paso (${var.<nombre>}). Dos hashes iguales son las mismas variables con los mismos valores; quien
	// decide guarda el de la última vez y los compara.
	HashDeLasVariables(ctx context.Context, intento string, ambito Ambito, textos []string) (string, error)
	// RegistrarProducido registra lo que produjo un comando: primero su hash en Historial, después su valor
	// por la relación reservada, y solo si las dos escrituras llegan, lo pliega en lo visible del intento.
	RegistrarProducido(ctx context.Context, intento, paso, nombre, valor string, ambito Ambito) error
	// NoReejecutado aporta, como producidas, las variables de la última vez que el paso se ejecutó de verdad
	// bajo ese ámbito — para un paso que no ejecuta ningún comando pero cuyo valor sigue haciendo falta. No
	// escribe nada: esos hashes y valores ya están en Historial de cuando se produjeron de verdad.
	NoReejecutado(ctx context.Context, intento, paso string, ambito Ambito) error
}

// ParaSimulacion es lo que usa Simulación de Pipeline: una petición sin intento (DEC-04.10). No calcula hash
// ni escribe nada en Historial — Declarar recibe ya resueltas las variables que Definición declaró, y
// RegistrarProducido solo se pliega en memoria.
type ParaSimulacion interface {
	// Declarar interpola y agrega los literales dados, en el ámbito de cada uno, y los deja visibles desde
	// ambito.
	Declarar(ctx context.Context, simulacion string, ambito Ambito, declaradas []VariableDeclarada) error
	// Interpolar sustituye cada ${var.<nombre>} por su valor entre lo visible desde ambito en esta simulación.
	Interpolar(ctx context.Context, simulacion string, ambito Ambito, texto string) (string, error)
	// RegistrarProducido pliega en memoria lo que un comando simulado dice que produciría.
	RegistrarProducido(ctx context.Context, simulacion, nombre, valor string, ambito Ambito) error
	// Cerrar libera la memoria de la simulación: una petición sin intento no deja rastro.
	Cerrar(ctx context.Context, simulacion string) error
}
