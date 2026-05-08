package dto

// ProjectInput contiene los datos de identificación del proyecto.
// `Url` y `Ref` son obligatorios desde M1: identifican el repo del proyecto a clonar.
type ProjectInput struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Team string `json:"team"`
	Org  string `json:"organization"`
	Url  string `json:"url"`
	Ref  string `json:"ref"`
}
