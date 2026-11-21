//go:build windows

package infraestructura

import (
	"os/exec"
	"time"
)

// prepararProceso en Windows solo acota la espera a que las tuberías se cierren: no hay grupos de procesos
// como en Unix, y acabar con el árbol entero necesitaría taskkill. Cancelar mata al shell, como siempre.
//
// En Windows no hay grupos de procesos: ni cancelar ni terminar un comando acaba con lo que lanzó. Es un límite
// conocido (AJ-08); el motor se ejecuta en contenedores Linux.
func prepararProceso(cmd *exec.Cmd, plazoDeGracia time.Duration) (terminar func()) {
	cmd.WaitDelay = plazoDeGracia + margenDeLaEspera
	return func() {}
}
