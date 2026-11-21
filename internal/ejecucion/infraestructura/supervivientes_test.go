package infraestructura

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSupervivientes_SiNoEsElPrimerProcesoNoMataNada(t *testing.T) {
	mato := false
	s := Supervivientes{
		esElPrimerProceso: func() bool { return false },
		acabarConLosDemas: func(time.Duration) error { mato = true; return nil },
	}

	require.NoError(t, s.Acabar(context.Background()))
	require.False(t, mato, "fuera del proceso 1, «todos los demás» no son del motor")
}

func TestSupervivientes_SiendoElPrimerProcesoAcabaConLosDemas(t *testing.T) {
	var plazo time.Duration
	s := Supervivientes{
		esElPrimerProceso: func() bool { return true },
		acabarConLosDemas: func(p time.Duration) error { plazo = p; return nil },
	}
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()

	require.NoError(t, s.Acabar(ctx), "también tras una cancelación")
	require.Equal(t, plazoParaRecogerSupervivientes, plazo)
}

func TestSupervivientes_SiNoPuedeAcabarConEllosLoDice(t *testing.T) {
	causa := errors.New("quedan procesos")
	s := Supervivientes{
		esElPrimerProceso: func() bool { return true },
		acabarConLosDemas: func(time.Duration) error { return causa },
	}

	require.ErrorIs(t, s.Acabar(context.Background()), causa)
}

// En la máquina de quien desarrolla y en CI las pruebas no son nunca el proceso 1: los de verdad no matan nada.
// Es lo que hace seguro construir NuevosSupervivientes en cualquier sitio.
func TestNuevosSupervivientes_FueraDelProceso1NoHacenNada(t *testing.T) {
	if esElPrimerProceso() {
		t.Skip("esta prueba es para cuando no se es el proceso 1")
	}
	require.NoError(t, NuevosSupervivientes().Acabar(context.Background()))
}
