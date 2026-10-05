package aplicacion

import (
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
	"github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
)

// Dependencias son los puertos del dominio que conecta la raíz de composición.
type Dependencias struct {
	Pipelines        dominio.Pipelines
	Fuentes          dominio.Fuentes
	Variables        dominio.Variables
	Historial        dominio.Historial
	Comandos         dominio.Comandos
	EspacioDeTrabajo dominio.EspacioDeTrabajo

	// Progreso es a dónde se cuenta cómo avanza un intento. Es opcional: sin él no se cuenta nada.
	Progreso dominio.Progreso

	// NombreDeLaHerramienta es la variable estándar tool_name (RD-04 §9, hallazgo 1): compartida, generada por
	// el motor, y por eso una dependencia — no una constante — para poder probarla.
	NombreDeLaHerramienta string
}

// Servicio es intentar: recorre EJ-1, o EJ-2 si trae un destino, con un bucle explícito sobre los pasos
// (docs/modelo/contextos/ejecucion.md, «Servicio de aplicación: intentar»). Coordina y no decide — la decisión
// es de dominio.DecidirPaso, y las reglas del desenlace son del agregado dominio.IntentoEnCurso.
type Servicio struct {
	d Dependencias
}

var _ publicado.ParaBorde = (*Servicio)(nil)

func NuevoServicio(d Dependencias) *Servicio {
	if d.Progreso == nil {
		d.Progreso = progresoNulo{}
	}
	return &Servicio{d: d}
}
