package dto

// ProjectInput contiene los datos de identificación del proyecto.
// Si URL está vacío, el orchestrator trata ID como una ruta local (modo legacy CLI).
type ProjectInput struct {
	ID   string
	Name string
	Team string
	Org  string
	URL  string
	Ref  string
}
