package aplicacion_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/publicado"
)

// Varios procesos escriben en el mismo almacén a la vez (cada contenedor es uno). La secuencia de lanzamientos es
// una sola para todos los ambientes, así que ahí es donde compiten de verdad: quien pierde vuelve a leer y a
// decidir, con una espera que crece y con azar, para que los perdedores no vuelvan a chocar todos juntos.
func TestRegistrarLanzamiento_MuchosALaVezTerminanTodosYNingunoSePierde(t *testing.T) {
	const lanzadores = 20
	h, ctx := nuevoHistorial()
	despliegues := make([]publicado.Despliegue, lanzadores)
	for i := range despliegues {
		_, despliegues[i], _ = intentar(t, h, ctx, apertura(fmt.Sprintf("amb-%d", i)), publicado.Exitoso, "")
	}

	errores := make([]error, lanzadores)
	var esperar sync.WaitGroup
	for i := range lanzadores {
		esperar.Add(1)
		go func() {
			defer esperar.Done()
			_, errores[i] = h.RegistrarLanzamiento(ctx, fmt.Sprintf("amb-%d", i), despliegues[i].Id, nada)
		}()
	}
	esperar.Wait()

	for i, err := range errores {
		require.NoError(t, err, "el lanzamiento %d", i)
	}
	todos, err := h.TodosLosLanzamientos(ctx)
	require.NoError(t, err)
	require.Len(t, todos, lanzadores)
	ambientes := map[string]bool{}
	for _, l := range todos {
		require.False(t, ambientes[l.Ambiente], "el lanzamiento de %s está dos veces", l.Ambiente)
		ambientes[l.Ambiente] = true
	}
	require.Len(t, ambientes, lanzadores, "ninguno se perdió")
}
