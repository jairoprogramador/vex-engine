package shared

import "time"

// Clock es el puerto por el que el dominio pregunta la hora (spec 07 §5.1).
//
// El paso del tiempo es una dependencia externa como el sistema de archivos o
// la red: el dominio la declara y la infraestructura la satisface. La regla que
// esto establece es que `time.Now()` desaparece del dominio —no de
// infraestructura, donde es legítimo— porque el instante de una ejecución es
// material de identidad (entra en `project_version` y de ahí en los metadatos
// del registro) y porque sin un reloj inyectable ninguna prueba de lo que se
// construya encima puede ser determinista.
type Clock interface {
	Now() time.Time
}

// FixedClock es un reloj detenido: devuelve siempre el mismo instante. Vive
// junto al puerto y no en un archivo _test.go porque lo usan las pruebas de
// varios paquetes.
type FixedClock struct {
	instant time.Time
}

func NewFixedClock(instant time.Time) FixedClock {
	return FixedClock{instant: instant}
}

func (c FixedClock) Now() time.Time {
	return c.instant
}

var _ Clock = (*FixedClock)(nil)
