package aplicacion

import (
	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
	"github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Historial dominio.Historial
}

// Servicio atiende lo que Diagnóstico publica hacia el borde (RD-10 lo conecta).
type Servicio struct {
	d Dependencias
}

var _ publicado.ParaBorde = (*Servicio)(nil)

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d}
}
