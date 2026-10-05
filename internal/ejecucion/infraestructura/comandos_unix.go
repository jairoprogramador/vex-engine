//go:build !windows

package infraestructura

import (
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// prepararProceso hace que el comando corra en su propio grupo de procesos, y que cancelarlo sea acabar con el
// grupo entero: primero SIGTERM, para que pueda terminar bien (terraform suelta el bloqueo de su estado), y
// pasado el plazo de gracia, SIGKILL. Sin esto, la cancelación solo mataba al shell: lo que el comando había
// lanzado seguía escribiendo en el espacio de trabajo, y si conservaba la tubería de salida, Run no volvía.
//
// Devuelve lo que hay que llamar cuando Run vuelve: apaga el temporizador y, si se canceló, acaba con lo que
// haya quedado en el grupo.
func prepararProceso(cmd *exec.Cmd, plazoDeGracia time.Duration) (terminar func(cancelado bool)) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = plazoDeGracia + margenDeLaEspera

	var cerrojo sync.Mutex
	var temporizador *time.Timer
	cmd.Cancel = func() error {
		grupo := -cmd.Process.Pid
		if err := syscall.Kill(grupo, syscall.SIGTERM); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			return err
		}
		cerrojo.Lock()
		defer cerrojo.Unlock()
		temporizador = time.AfterFunc(plazoDeGracia, func() { _ = syscall.Kill(grupo, syscall.SIGKILL) })
		return nil
	}

	return func(cancelado bool) {
		cerrojo.Lock()
		defer cerrojo.Unlock()
		if temporizador != nil {
			temporizador.Stop()
		}
		if cancelado && cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}
}
