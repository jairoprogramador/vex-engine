package aplicacion

import (
	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Pipelines       dominio.Pipelines
	Variables       dominio.Variables
	EspacioTemporal dominio.EspacioTemporal
}

// Servicio es simular: recorre SIM-1 sin efectos (docs/modelo/contextos/simulacion.md, «Servicio de
// aplicación»). Coordina y no decide — fabricar una salida simulada es dominio.FabricarSalidaSimulada.
type Servicio struct {
	d Dependencias
}

var _ publicado.ParaBorde = (*Servicio)(nil)

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d}
}
