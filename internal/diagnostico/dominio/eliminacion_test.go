package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
)

func comparacionDePrueba(t *testing.T, cambioCodigo, cambioInstrucciones, cambioVariables bool) dominio.Comparacion {
	t.Helper()
	pasos := []dominio.EstadoDePaso{{
		Paso: nombrePaso(t, "deploy"), CambioCodigo: cambioCodigo,
		CambioInstrucciones: cambioInstrucciones, CambioVariables: cambioVariables,
	}}
	return dominio.NuevaComparacion(dominio.Referencia{}, pasos, nil)
}

func TestEliminacion(t *testing.T) {
	t.Run("una comparación descarta los ejes que no cambiaron", func(t *testing.T) {
		// ES-1: instrucciones no cambiaron, código y variables sí — quedan dos candidatos.
		comparacion := comparacionDePrueba(t, true, false, true)
		atribucion := dominio.Eliminacion([]dominio.Comparacion{comparacion})

		require.True(t, atribucion.Tiene(dominio.Codigo))
		require.False(t, atribucion.Tiene(dominio.Instrucciones))
		require.True(t, atribucion.Tiene(dominio.Variables))
	})

	t.Run("dos comparaciones combinadas: el caso normal deja un solo candidato", func(t *testing.T) {
		// ES-1: código y variables cambiaron, instrucciones no.
		es1 := comparacionDePrueba(t, true, false, true)
		// ES-2: código e instrucciones no cambiaron (mismo hash), variables sí.
		es2 := comparacionDePrueba(t, false, false, true)

		atribucion := dominio.Eliminacion([]dominio.Comparacion{es1, es2})

		require.Equal(t, []dominio.Eje{dominio.Variables}, atribucion.Ejes())
	})

	t.Run("los tres ejes descartados es ES-5, un hecho, no un error", func(t *testing.T) {
		comparacion := comparacionDePrueba(t, false, false, false)
		atribucion := dominio.Eliminacion([]dominio.Comparacion{comparacion})

		require.True(t, atribucion.Vacia())
		require.Empty(t, atribucion.Ejes())
	})

	t.Run("ningún eje descartado deja los tres candidatos", func(t *testing.T) {
		comparacion := comparacionDePrueba(t, true, true, true)
		atribucion := dominio.Eliminacion([]dominio.Comparacion{comparacion})

		require.Equal(t, dominio.TodosLosEjes(), atribucion.Ejes())
	})
}
