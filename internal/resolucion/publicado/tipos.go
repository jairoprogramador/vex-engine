package publicado

// Ambito es compartido o el de un ambiente concreto.
type Ambito struct {
	Compartido bool
	Ambiente   string // vacío si es compartido
}

// Origen dice de dónde salió una variable: de un literal del pipeline o de lo que produjo un comando.
type Origen string

const (
	Declarada Origen = "declarada"
	Producida Origen = "producida"
)

// Variable es una variable efectiva, sin su valor (IT-04 DEC-04.7): el valor solo sale hacia interpolación y
// hacia la relación reservada de Historial, nunca hacia lo publicado.
type Variable struct {
	Nombre string
	Origen Origen
	Ambito Ambito
}

// VariableDeclarada es un literal del pipeline, con su valor sin interpolar. Lo usa Simulación, que ya lo
// trae resuelto desde Definición sin pasar por Historial (DEC-04.10).
type VariableDeclarada struct {
	Nombre string
	Ambito Ambito
	Valor  string
}
