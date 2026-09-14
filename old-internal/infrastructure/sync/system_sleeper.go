package sync

import (
	"context"
	"time"

	domSync "github.com/jairoprogramador/vex-engine/old-internal/domain/sync"
)

var _ domSync.Sleeper = (*SystemSleeper)(nil)

// SystemSleeper es la espera de verdad, y es infraestructura por lo mismo que el
// reloj: `time.Sleep` dentro del dominio haría que probar una política de
// reintento costara lo que la política dice.
//
// Respeta la cancelación. Un `Ctrl-C` a mitad de un backoff tiene que llegar al
// proceso: el registro no puede retener una interrupción, igual que no puede
// retener un despliegue.
type SystemSleeper struct{}

func NewSystemSleeper() domSync.Sleeper { return &SystemSleeper{} }

func (s *SystemSleeper) Sleep(ctx *context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}

	timer := time.NewTimer(duration)
	defer timer.Stop()

	if ctx == nil || *ctx == nil {
		<-timer.C
		return nil
	}

	select {
	case <-(*ctx).Done():
		return (*ctx).Err()
	case <-timer.C:
		return nil
	}
}
