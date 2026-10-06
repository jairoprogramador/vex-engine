package aplicacion

import (
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/dominio"
	"github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Historial dominio.Historial
}

// Servicio atiende lo que Lanzamiento publica hacia el borde (LAN-2, LAN-3) y lo que escucha del Historial
// (LAN-1, en infraestructura/escucha.go).
type Servicio struct {
	d Dependencias
}

var _ publicado.ParaBorde = (*Servicio)(nil)

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d}
}
