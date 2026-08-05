package shared

import (
	"time"

	domShared "github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

// SystemClock es el adaptador real del puerto Clock: el reloj del proceso.
// Es el único sitio de todo el motor donde el instante actual se obtiene de
// `time` para alimentar al dominio (spec 07 §5.1).
type SystemClock struct{}

func NewSystemClock() SystemClock {
	return SystemClock{}
}

func (SystemClock) Now() time.Time {
	return time.Now()
}

var _ domShared.Clock = (*SystemClock)(nil)
