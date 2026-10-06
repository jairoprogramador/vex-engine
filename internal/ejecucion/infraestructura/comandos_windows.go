//go:build windows

package infraestructura

import (
	"os/exec"
	"time"
)

// prepararProceso en Windows solo acota la espera a que las tuberías se cierren: no hay grupos de procesos
// como en Unix, y acabar con el árbol entero necesitaría taskkill. Cancelar mata al shell, como siempre.
func prepararProceso(cmd *exec.Cmd, plazoDeGracia time.Duration) (terminar func(cancelado bool)) {
	cmd.WaitDelay = plazoDeGracia + margenDeLaEspera
	return func(bool) {}
}
