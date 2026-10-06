package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
)

func TestCompararPasos(t *testing.T) {
	t.Run("nada cambió, ningún eje cambia", func(t *testing.T) {
		delQueFalla := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "deploy", "c1", "i1", map[string]string{"v": "h1"})}
		deLaReferencia := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "deploy", "c1", "i1", map[string]string{"v": "h1"})}

		estados := dominio.CompararPasos(delQueFalla, deLaReferencia)
		require.Len(t, estados, 1)
		require.False(t, estados[0].CambioCodigo)
		require.False(t, estados[0].CambioInstrucciones)
		require.False(t, estados[0].CambioVariables)
	})

	t.Run("distinto hash del código", func(t *testing.T) {
		delQueFalla := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "deploy", "c2", "i1", nil)}
		deLaReferencia := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "deploy", "c1", "i1", nil)}

		estados := dominio.CompararPasos(delQueFalla, deLaReferencia)
		require.True(t, estados[0].CambioCodigo)
		require.False(t, estados[0].CambioInstrucciones)
	})

	t.Run("distinto hash de instrucciones", func(t *testing.T) {
		delQueFalla := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "deploy", "c1", "i2", nil)}
		deLaReferencia := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "deploy", "c1", "i1", nil)}

		estados := dominio.CompararPasos(delQueFalla, deLaReferencia)
		require.False(t, estados[0].CambioCodigo)
		require.True(t, estados[0].CambioInstrucciones)
	})

	t.Run("una variable declarada distinta", func(t *testing.T) {
		delQueFalla := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "deploy", "c1", "i1", map[string]string{"v": "h2"})}
		deLaReferencia := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "deploy", "c1", "i1", map[string]string{"v": "h1"})}

		estados := dominio.CompararPasos(delQueFalla, deLaReferencia)
		require.True(t, estados[0].CambioVariables)
		require.Equal(t, []dominio.NombreDeVariable{nombreDeVariable(t, "v")}, estados[0].VariablesDeclaradasCambiadas)
	})

	t.Run("un paso presente en un solo lado cuenta como cambio de instrucciones", func(t *testing.T) {
		delQueFalla := []dominio.EjesDeUnPaso{ejesDeUnPasoDePrueba(t, "nuevo", "c1", "i1", nil)}

		estados := dominio.CompararPasos(delQueFalla, nil)
		require.True(t, estados[0].CambioInstrucciones)
		require.False(t, estados[0].CambioCodigo)
	})
}

func TestComparacion_Descarta(t *testing.T) {
	pasos := []dominio.EstadoDePaso{{Paso: nombrePaso(t, "deploy"), CambioCodigo: false, CambioInstrucciones: true, CambioVariables: false}}
	comparacion := dominio.NuevaComparacion(dominio.Referencia{}, pasos, nil)

	require.True(t, comparacion.Descarta(dominio.Codigo), "no cambió: se descarta")
	require.False(t, comparacion.Descarta(dominio.Instrucciones), "cambió: no se descarta")
	require.True(t, comparacion.Descarta(dominio.Variables), "no cambió: se descarta")
}

func TestCompararProducidas(t *testing.T) {
	producida := func(paso string, valores map[string]string) dominio.ProducidasDeUnPaso {
		mapa := map[dominio.NombreDeVariable]dominio.HashDeVariable{}
		for nombre, hash := range valores {
			mapa[nombreDeVariable(t, nombre)] = hashDeVariable(t, hash)
		}
		return dominio.NuevasProducidasDeUnPaso(nombrePaso(t, paso), mapa)
	}

	t.Run("una producida cambiada se reporta", func(t *testing.T) {
		delQueFalla := []dominio.ProducidasDeUnPaso{producida("deploy", map[string]string{"url": "h2"})}
		deLaReferencia := []dominio.ProducidasDeUnPaso{producida("deploy", map[string]string{"url": "h1"})}

		cambios := dominio.CompararProducidas(delQueFalla, deLaReferencia)
		require.Equal(t, []dominio.CambioDeVariable{{Paso: nombrePaso(t, "deploy"), Nombre: nombreDeVariable(t, "url")}}, cambios)
	})

	t.Run("sin cambios no reporta nada", func(t *testing.T) {
		delQueFalla := []dominio.ProducidasDeUnPaso{producida("deploy", map[string]string{"url": "h1"})}
		deLaReferencia := []dominio.ProducidasDeUnPaso{producida("deploy", map[string]string{"url": "h1"})}

		require.Empty(t, dominio.CompararProducidas(delQueFalla, deLaReferencia))
	})
}
