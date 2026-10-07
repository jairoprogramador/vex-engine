package publicado

// PeticionDeCatalogo dice de qué pipeline se pregunta: el de una fuente, de hoy si Commit está vacío, o el de
// ese commit.
type PeticionDeCatalogo struct {
	Version           string // DEC-05.6
	FuenteDelPipeline string
	Commit            string
}

// Ambiente es un ambiente del pipeline, en su orden, y si el dueño del negocio se lo reservó.
type Ambiente struct {
	Nombre      string
	Descripcion string
	// Valor es con el que se nombra el ambiente al pedir un intento o un lanzamiento.
	Valor     string
	Reservado bool
}

// Paso es un paso del pipeline, en su orden.
type Paso struct {
	Nombre string
	Orden  int
	// Compartido: su ámbito es el compartido, no el de un ambiente.
	Compartido bool
}
