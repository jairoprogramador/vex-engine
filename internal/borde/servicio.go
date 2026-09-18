package borde

import (
	diagnosticopublicado "github.com/jairoprogramador/vex-engine/internal/diagnostico/publicado"
	ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"
	historialpublicado "github.com/jairoprogramador/vex-engine/internal/historial/publicado"
	lanzamientopublicado "github.com/jairoprogramador/vex-engine/internal/lanzamiento/publicado"
	simulacionpublicado "github.com/jairoprogramador/vex-engine/internal/simulacion/publicado"
)

// Dependencias son los contextos de entrada, cada uno por lo que publica hacia el borde. Los conecta la raíz
// de composición.
type Dependencias struct {
	Ejecucion   ejecucionpublicado.ParaBorde
	Simulacion  simulacionpublicado.ParaBorde
	Lanzamiento lanzamientopublicado.ParaBorde
	Diagnostico diagnosticopublicado.ParaBorde
	Historial   historialpublicado.ParaBorde
}

// Servicio es el Open Host del motor (RD-10): recibe una petición por invocación, comprueba su versión del
// lenguaje publicado (DEC-05.6) y la envía al contexto de entrada que la atiende. No decide nada de negocio.
type Servicio struct {
	d Dependencias
}

func NuevoServicio(d Dependencias) *Servicio {
	return &Servicio{d: d}
}
