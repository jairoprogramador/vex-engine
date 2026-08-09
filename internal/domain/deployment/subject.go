package deployment

import "fmt"

// Subject es el proyecto: la url de su repositorio.
//
// Es el MISMO valor que `cache.Material.Subject` y que `state.Key.subject`, y
// eso no es una coincidencia que convenga romper: las tres direcciones —caché,
// estado y objeto de despliegue— hablan del mismo proyecto, y si divergieran, el
// registro de un despliegue no se podría cruzar con el estado que dejó.
//
// Aquí NO se re-valida la forma de la url. Quien la construye ya pasó por
// `shared.RepositoryUrl` en el borde, y volver a validarla aquí produciría dos
// definiciones de «url válida» que envejecen por separado. Lo que sí se exige es
// que exista: un sujeto vacío da una identidad válida que colisiona con la de
// cualquier otro contenido al que también le falte, y esa colisión es permanente.
type Subject struct {
	url string
}

// NewSubject construye el sujeto.
func NewSubject(url string) (Subject, error) {
	if url == "" {
		return Subject{}, fmt.Errorf("deployment: el contenido no tiene sujeto (url del proyecto)")
	}
	return Subject{url: url}, nil
}

// String es la url del proyecto, tal cual entra en el material canónico.
func (s Subject) String() string { return s.url }

// IsZero indica que no hay sujeto.
func (s Subject) IsZero() bool { return s.url == "" }

// Equals es la regla de igualdad del value object.
func (s Subject) Equals(other Subject) bool { return s.url == other.url }
