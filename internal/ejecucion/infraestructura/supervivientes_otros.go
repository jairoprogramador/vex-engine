//go:build !linux

package infraestructura

import "time"

// Fuera de Linux el motor no es nunca el proceso 1 de un contenedor: no se mata nada (ver Supervivientes).
func esElPrimerProceso() bool { return false }

func acabarConLosDemasProcesos(time.Duration) error { return nil }
