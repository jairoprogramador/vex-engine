package dominio

import "context"

// Historial es lo que la aplicación necesita de Historial de Cambios, en el lenguaje de este dominio: nunca
// intentos, nunca evidencias — solo hashes y valores de un paso, con el salto de evidencia ya resuelto por
// quien implemente este puerto.
type Historial interface {
	// RegistrarVariable guarda el hash, el origen y el ámbito de una variable que se acaba de producir. El
	// ámbito viaja en el mismo registro porque es lo único que permite reconstruirlo después: la relación
	// reservada solo guarda el valor, sin metadato ninguno.
	RegistrarVariable(ctx context.Context, intento, paso, nombre string, hash HashDeVariable, origen Origen, ambito Ambito) error
	// GuardarValor es de la relación reservada: el valor en claro, y nada más.
	GuardarValor(ctx context.Context, intento, paso, nombre, valor string) error
	// ValoresDeLaUltimaVez es lo mismo, pero con el valor en claro (relación reservada) y el ámbito con el que
	// se produjo cada una — para un paso que no se re-ejecuta y aporta lo de su última vez.
	ValoresDeLaUltimaVez(ctx context.Context, paso string, ambito Ambito) (map[string]ValorDeLaUltimaVez, bool, error)
}

// ValorDeLaUltimaVez es el valor en claro de una variable producida la última vez, con el ámbito bajo el que
// se produjo entonces (una variable compartida sigue siendo compartida al recuperarla).
type ValorDeLaUltimaVez struct {
	Valor  string
	Ambito Ambito
}

// Definicion es lo que la aplicación necesita de Definición de Pipeline.
type Definicion interface {
	// VariablesDeclaradas son los literales del pipeline de ese commit, cada uno con su ámbito. Su orden no es
	// de dependencia (RD-04 §9.16): un literal puede usar otro declarado después que él en esta lista.
	VariablesDeclaradas(ctx context.Context, fuente, commit string) ([]VariableDeclarada, error)
	// VariablesEstandar son las que un pipeline usa sin declararlas. No hace falta ctx: Definición las tiene ya
	// resueltas en memoria, sin I/O.
	VariablesEstandar() []VariableEstandar
}

// VariableDeclarada es un literal del pipeline, tal como Definición lo publica, traducido al ámbito de este
// dominio.
type VariableDeclarada struct {
	Nombre string
	Ambito Ambito
	Valor  string
}

// VariableEstandar es una de las variables que el pipeline usa sin declararla. Definición solo dice su nombre
// y su comportamiento; el valor lo da quien invoca a Resolución (RD-06 todavía no existe para dárselo de otra
// forma). DelPaso decide su ámbito: si es de cada paso, el del paso que la usa; si no, el compartido.
type VariableEstandar struct {
	Nombre   string
	Metadato bool
	DelPaso  bool
}
