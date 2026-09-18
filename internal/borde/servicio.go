package borde

import ejecucionpublicado "github.com/jairoprogramador/vex-engine/internal/ejecucion/publicado"

// Servicio es el borde mínimo de RD-06: recibe una petición por invocación, comprueba su versión del lenguaje
// publicado (DEC-05.6) y la envía a Ejecución de Pipeline. RD-10 completa el resto de operaciones sobre el
// mismo Servicio.
type Servicio struct {
	ejecucion ejecucionpublicado.ParaBorde
}

func NuevoServicio(ejecucion ejecucionpublicado.ParaBorde) *Servicio {
	return &Servicio{ejecucion: ejecucion}
}
