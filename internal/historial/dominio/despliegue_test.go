package dominio_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

func TestDesplegar_SoloNaceDeUnIntentoExitosoConCommitsYTodosLosPasos(t *testing.T) {
	previo := abierto(t, "i0", aperturaEn("staging"))
	hacer(t, previo, 0, "supply")

	casos := map[string]struct {
		preparar func(t *testing.T) *dominio.Intento
		nace     bool
	}{
		"exitoso, con commits y todos los pasos": {func(t *testing.T) *dominio.Intento {
			i := abierto(t, "i1", aperturaEn("staging"))
			hacer(t, i, 1, "test", "supply", "deploy")
			cerrar(t, i, dominio.Exitoso)
			return i
		}, true},
		"con un paso que no se re-ejecutó": {func(t *testing.T) *dominio.Intento {
			i := abierto(t, "i1", aperturaEn("staging"))
			hacer(t, i, 1, "test")
			require.NoError(t, i.NoReejecutar("supply", dominio.Evidencia{Intento: "i0", Paso: "supply"}, previo, en(2), nada))
			hacer(t, i, 3, "deploy")
			cerrar(t, i, dominio.Exitoso)
			return i
		}, true},
		"fallido": {func(t *testing.T) *dominio.Intento {
			i := abierto(t, "i1", aperturaEn("staging"))
			hacer(t, i, 1, "test", "supply", "deploy")
			cerrar(t, i, dominio.Fallido)
			return i
		}, false},
		"cancelado": {func(t *testing.T) *dominio.Intento {
			i := abierto(t, "i1", aperturaEn("staging"))
			cerrar(t, i, dominio.Cancelado)
			return i
		}, false},
		"con una copia de trabajo": {func(t *testing.T) *dominio.Intento {
			a := aperturaEn("staging")
			a.ConCommits = false
			i := abierto(t, "i1", a)
			hacer(t, i, 1, "test", "supply", "deploy")
			cerrar(t, i, dominio.Exitoso)
			return i
		}, false},
		"sin pedir todos los pasos del pipeline": {func(t *testing.T) *dominio.Intento {
			a := aperturaEn("staging")
			a.HastaPaso = "supply"
			i := abierto(t, "i1", a)
			hacer(t, i, 1, "test", "supply")
			cerrar(t, i, dominio.Exitoso)
			return i
		}, false},
		"sin cierre": {func(t *testing.T) *dominio.Intento {
			i := abierto(t, "i1", aperturaEn("staging"))
			hacer(t, i, 1, "test", "supply", "deploy")
			return i
		}, false},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			i := c.preparar(t)
			despliegues := dominio.NuevosDesplieguesDeUnAmbiente("staging")

			d, hay, err := i.Desplegar(despliegues, "d1", en(10))
			require.NoError(t, err)
			require.Equal(t, c.nace, hay)
			if c.nace {
				require.Equal(t, i.Id(), d.Intento())
				require.Len(t, despliegues.Nuevos(), 1)
			} else {
				require.Empty(t, despliegues.Nuevos())
			}
		})
	}
}

func completo(
	t *testing.T, id dominio.IdIntento, despliegues *dominio.DesplieguesDeUnAmbiente, nuevo dominio.IdDespliegue,
	cierre dominio.Cierre,
) dominio.Despliegue {
	t.Helper()
	i := abierto(t, id, aperturaEn(despliegues.Ambiente()))
	hacer(t, i, 1, "test", "supply", "deploy")
	require.NoError(t, i.Cerrar(cierre, en(2), despliegues))
	d, hay, err := i.Desplegar(despliegues, nuevo, en(2))
	require.NoError(t, err)
	require.True(t, hay)
	return d
}

func TestDesplegar_ElPadreEsElUltimoOElDestinoDelRollback(t *testing.T) {
	despliegues := dominio.NuevosDesplieguesDeUnAmbiente("staging")

	d1 := completo(t, "i1", despliegues, "d1", dominio.Cierre{Estado: dominio.Exitoso})
	d2 := completo(t, "i2", despliegues, "d2", dominio.Cierre{Estado: dominio.Exitoso})
	d3 := completo(t, "i3", despliegues, "d3", dominio.Cierre{Estado: dominio.Exitoso, Destino: "d1"})

	require.Empty(t, d1.Padre(), "el primero no tiene padre")
	require.Equal(t, dominio.IdDespliegue("d1"), d2.Padre())
	require.Equal(t, dominio.IdDespliegue("d1"), d3.Padre(), "dos despliegues pueden compartir padre")
	ultimo, _ := despliegues.Ultimo()
	require.Equal(t, d3, ultimo)
}

func TestDesplegar_UnIntentoTieneComoMuchoUnDespliegue(t *testing.T) {
	despliegues := dominio.NuevosDesplieguesDeUnAmbiente("staging")
	i := abierto(t, "i1", aperturaEn("staging"))
	hacer(t, i, 1, "test", "supply", "deploy")
	cerrar(t, i, dominio.Exitoso)

	primero, _, err := i.Desplegar(despliegues, "d1", en(2))
	require.NoError(t, err)
	otra, hay, err := i.Desplegar(despliegues, "d2", en(3))
	require.NoError(t, err)
	require.True(t, hay)
	require.Equal(t, primero, otra)
	require.Len(t, despliegues.Nuevos(), 1)
}

func TestReconstituirDesplieguesDeUnAmbiente_CompruebaSusInvariantes(t *testing.T) {
	d1 := dominio.ReconstituirDespliegue("d1", "staging", "i1", "", en(1))

	casos := map[string][]dominio.Despliegue{
		"el primero con padre":        {dominio.ReconstituirDespliegue("d1", "staging", "i1", "d0", en(1))},
		"el segundo sin padre":        {d1, dominio.ReconstituirDespliegue("d2", "staging", "i2", "", en(2))},
		"de otro ambiente":            {dominio.ReconstituirDespliegue("d1", "prod", "i1", "", en(1))},
		"un padre que no es anterior": {d1, dominio.ReconstituirDespliegue("d2", "staging", "i2", "d9", en(2))},
		"dos de un mismo intento":     {d1, dominio.ReconstituirDespliegue("d2", "staging", "i1", "d1", en(2))},
		"sin instante":                {dominio.ReconstituirDespliegue("d1", "staging", "i1", "", time.Time{})},
	}
	for nombre, despliegues := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := dominio.ReconstituirDesplieguesDeUnAmbiente("staging", despliegues)
			require.ErrorIs(t, err, dominio.ErrRechazado)
		})
	}

	d2 := dominio.ReconstituirDespliegue("d2", "staging", "i2", "d1", en(2))
	leidos, err := dominio.ReconstituirDesplieguesDeUnAmbiente("staging", []dominio.Despliegue{d1, d2})
	require.NoError(t, err)
	require.Equal(t, 2, leidos.Leidos())
	require.Empty(t, leidos.Nuevos())
}
