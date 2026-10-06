package aplicacion

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// sinEsperar quita la espera entre vueltas: lo que se prueba es la decisión de reintentar, no el reloj.
func sinEsperar(t *testing.T) {
	t.Helper()
	anterior := esperaBase
	esperaBase = 0
	t.Cleanup(func() { esperaBase = anterior })
}

func conflicto() error { return fmt.Errorf("al añadir: %w", dominio.ErrConflicto) }

func TestConReintento_UnConflictoSeReintentaHastaQueSePuedeEscribir(t *testing.T) {
	sinEsperar(t)
	llamadas := 0

	err := conReintento(context.Background(), func() error {
		llamadas++
		if llamadas < 7 {
			return conflicto()
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 7, llamadas, "más de las 3 vueltas de antes: con muchos escritores el último necesita muchas")
}

func TestConReintento_SiSeAgotanLasVueltasSeDiceQueFueLaConcurrenciaYNoSeEscribioNada(t *testing.T) {
	sinEsperar(t)
	llamadas := 0

	err := conReintento(context.Background(), func() error { llamadas++; return conflicto() })

	require.Equal(t, vueltasAnteConflicto, llamadas)
	require.ErrorIs(t, err, dominio.ErrEscrituraConcurrente)
	require.ErrorIs(t, err, dominio.ErrConflicto, "y sigue diciendo por qué")
	require.ErrorIs(t, traducir(err), publicado.ErrEscrituraConcurrente, "quien lo recibe puede distinguirlo de un fallo")
}

func TestConReintento_LoQueNoEsUnConflictoNoSeReintenta(t *testing.T) {
	sinEsperar(t)
	llamadas := 0
	roto := errors.New("el disco se llenó")

	err := conReintento(context.Background(), func() error { llamadas++; return roto })

	require.ErrorIs(t, err, roto)
	require.Equal(t, 1, llamadas)
}

func TestConReintento_SiSeCancelaMientrasEsperaSeDetiene(t *testing.T) {
	anterior := esperaBase
	esperaBase = time.Hour // tope enorme: solo la cancelación puede terminar la espera
	t.Cleanup(func() { esperaBase = anterior })
	ctx, cancelar := context.WithCancel(context.Background())
	llamadas := 0

	terminado := make(chan error, 1)
	go func() {
		terminado <- conReintento(ctx, func() error { llamadas++; cancelar(); return conflicto() })
	}()

	select {
	case err := <-terminado:
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, 1, llamadas)
	case <-time.After(5 * time.Second):
		t.Fatal("la espera no respetó la cancelación")
	}
}

func TestEsperarAnteConflicto_CrecePeroTieneTope(t *testing.T) {
	anterior := esperaBase
	esperaBase = time.Millisecond
	t.Cleanup(func() { esperaBase = anterior })

	inicio := time.Now()
	for vuelta := 1; vuelta <= vueltasAnteConflicto; vuelta++ {
		require.NoError(t, esperarAnteConflicto(context.Background(), vuelta))
	}

	// Con topes de 1, 2, 4… ms (y 250 ms como máximo) y azar entre cero y el tope, el total no pasa de la suma.
	require.Less(t, time.Since(inicio), 2*time.Second)
}
