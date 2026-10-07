package publicado

import "time"

// Lanzamiento es un despliegue que se hizo visible, con su versión y su nombre.
type Lanzamiento struct {
	// Id es la identidad del lanzamiento, la que decide el Historial al registrarlo.
	Id         string
	Ambiente   string
	Despliegue string
	Version    int
	Nombre     string
	Instante   time.Time
}
