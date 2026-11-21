//go:build linux

package infraestructura

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

func esElPrimerProceso() bool { return os.Getpid() == 1 }

// acabarConLosDemasProcesos solo se llama siendo el proceso 1. kill(-1) manda la señal a todos los procesos del
// espacio menos al que la manda y al proceso 1, que aquí son el mismo; el núcleo lo hace sin que ninguno pueda
// crear otro entre medias. Después recoge a los que mueren: al morir su padre pasan a ser hijos del proceso 1, y
// sin recogerlos quedarían como zombis. No hay comandos en curso entre dos pasos, así que no se le quita a nadie
// el estado de un hijo que esté esperando.
func acabarConLosDemasProcesos(plazo time.Duration) error {
	if err := syscall.Kill(-1, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("kill(-1): %w", err) // ESRCH: no había nadie más
	}
	limite := time.Now().Add(plazo)
	for {
		var estado syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &estado, syscall.WNOHANG, nil)
		switch {
		case errors.Is(err, syscall.ECHILD):
			return nil // no queda ningún hijo, ni vivo ni por recoger
		case errors.Is(err, syscall.EINTR), pid > 0:
			continue
		case err != nil:
			return fmt.Errorf("wait4(-1): %w", err)
		case time.Now().After(limite):
			return fmt.Errorf("pasados %s quedan procesos que no han muerto", plazo)
		}
		time.Sleep(time.Millisecond)
	}
}
