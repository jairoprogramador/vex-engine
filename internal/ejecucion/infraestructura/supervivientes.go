package infraestructura

import (
	"context"
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// plazoParaRecogerSupervivientes es lo que se espera a que los procesos a los que se mató terminen de morir.
// SIGKILL no se puede ignorar: si pasado este plazo queda alguno, algo va mal y el paso no queda limpio.
const plazoParaRecogerSupervivientes = 5 * time.Second

// Supervivientes acaba, entre paso y paso, con todos los procesos que no son el motor. Solo donde eso es seguro
// y posible: en Linux y cuando el motor es el proceso 1 de su espacio de procesos, que es como corre en su
// contenedor (ENTRYPOINT ["vexd"], sin --init). Ahí «todos los demás» son, por construcción, lo que lanzaron los
// comandos: también lo que escapó de su grupo con setsid, que es lo que prepararProceso no alcanza.
//
// En cualquier otro caso —otro sistema, el motor lanzado a mano, un contenedor con --init, donde el proceso 1
// es otro— no hace nada: matar «todos los demás» ahí sería matar procesos que no son del motor. El límite queda
// como estaba: lo que cree su propia sesión sobrevive a su comando y a su paso (AJ-08).
type Supervivientes struct {
	// esElPrimerProceso dice si el motor es el proceso 1. acabarConLosDemas mata a todos los demás y espera a
	// que mueran. Son campos para poder probar la decisión sin matar nada.
	esElPrimerProceso func() bool
	acabarConLosDemas func(plazo time.Duration) error
}

var _ dominio.Supervivientes = Supervivientes{}

// NuevosSupervivientes son los de la plataforma para la que se compiló el motor.
func NuevosSupervivientes() Supervivientes {
	return Supervivientes{esElPrimerProceso: esElPrimerProceso, acabarConLosDemas: acabarConLosDemasProcesos}
}

// Acabar mata lo que quede vivo, si el motor es el proceso 1. No depende de ctx: se llama también tras una
// cancelación, y es corto.
func (s Supervivientes) Acabar(context.Context) error {
	if !s.esElPrimerProceso() {
		return nil
	}
	if err := s.acabarConLosDemas(plazoParaRecogerSupervivientes); err != nil {
		return fmt.Errorf("ejecución: acabar con los procesos que dejó el paso: %w", err)
	}
	return nil
}
