package dominio_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

var t0 = time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

func en(minuto int) time.Time { return t0.Add(time.Duration(minuto) * time.Minute) }

var nada = dominio.Contenido{}

func aperturaEn(ambiente dominio.Ambiente) dominio.Apertura {
	return dominio.Apertura{
		Ambiente:    ambiente,
		Solicitante: "ana",
		Pasos: []dominio.PasoDeclarado{
			{Nombre: "test"}, {Nombre: "supply", Compartido: true}, {Nombre: "deploy"},
		},
		HastaPaso:     "deploy",
		ConCommits:    true,
		HashDelCodigo: "h1",
	}
}

func abierto(t *testing.T, id dominio.IdIntento, a dominio.Apertura) *dominio.Intento {
	t.Helper()
	i := dominio.NuevoIntento(id)
	require.NoError(t, i.Abrir(a, en(0), nada))
	return i
}

func hacer(t *testing.T, i *dominio.Intento, minuto int, pasos ...dominio.NombrePaso) {
	t.Helper()
	for _, p := range pasos {
		require.NoError(t, i.Comenzar(p, en(minuto), nada))
		require.NoError(t, i.Terminar(p, true, en(minuto), nada))
	}
}

func cerrar(t *testing.T, i *dominio.Intento, estado dominio.Estado) {
	t.Helper()
	a, _ := i.Apertura()
	require.NoError(t, i.Cerrar(dominio.Cierre{Estado: estado}, en(9), dominio.NuevosDesplieguesDeUnAmbiente(a.Ambiente)))
}

func TestIntento_LaAperturaEsUnaYEsLaPrimera(t *testing.T) {
	sinAbrir := dominio.NuevoIntento("i1")
	require.ErrorIs(t, sinAbrir.Comenzar("test", en(1), nada), dominio.ErrRechazado)

	i := abierto(t, "i1", aperturaEn("staging"))
	require.ErrorIs(t, i.Abrir(aperturaEn("staging"), en(1), nada), dominio.ErrRechazado)
}

func TestIntento_UnaAperturaIncompletaSeRechaza(t *testing.T) {
	casos := map[string]func(*dominio.Apertura){
		"sin ambiente":        func(a *dominio.Apertura) { a.Ambiente = "" },
		"sin solicitante":     func(a *dominio.Apertura) { a.Solicitante = "" },
		"sin hash del código": func(a *dominio.Apertura) { a.HashDelCodigo = "" },
		"sin pasos":           func(a *dominio.Apertura) { a.Pasos = nil },
		"un paso sin nombre":  func(a *dominio.Apertura) { a.Pasos[0].Nombre = "" },
		"un paso repetido":    func(a *dominio.Apertura) { a.Pasos[1].Nombre = "test" },
		"pide un paso ajeno":  func(a *dominio.Apertura) { a.HastaPaso = "notify" },
	}
	for nombre, estropear := range casos {
		t.Run(nombre, func(t *testing.T) {
			a := aperturaEn("staging")
			estropear(&a)
			require.ErrorIs(t, dominio.NuevoIntento("i1").Abrir(a, en(0), nada), dominio.ErrRechazado)
		})
	}
}

func TestIntento_SoloRegistraLosPasosPedidos(t *testing.T) {
	a := aperturaEn("staging")
	a.HastaPaso = "test"
	i := abierto(t, "i1", a)

	require.ErrorIs(t, i.Comenzar("supply", en(1), nada), dominio.ErrRechazado, "después del pedido")
	require.ErrorIs(t, i.Comenzar("notify", en(1), nada), dominio.ErrRechazado, "no es del pipeline")
	require.NoError(t, i.Comenzar("test", en(1), nada))
}

func TestIntento_UnPasoTieneUnComienzoYUnFinalOUnaNoReejecucion(t *testing.T) {
	previo := abierto(t, "i0", aperturaEn("staging"))
	hacer(t, previo, 1, "test")
	evidencia := dominio.Evidencia{Intento: "i0", Paso: "test"}

	casos := map[string]func(*dominio.Intento) error{
		"final sin comienzo": func(i *dominio.Intento) error {
			return i.Terminar("test", true, en(2), nada)
		},
		"dos comienzos": func(i *dominio.Intento) error {
			require.NoError(t, i.Comenzar("test", en(2), nada))
			return i.Comenzar("test", en(3), nada)
		},
		"dos finales": func(i *dominio.Intento) error {
			require.NoError(t, i.Comenzar("test", en(2), nada))
			require.NoError(t, i.Terminar("test", false, en(3), nada))
			return i.Terminar("test", true, en(4), nada)
		},
		"no re-ejecución después de un comienzo": func(i *dominio.Intento) error {
			require.NoError(t, i.Comenzar("test", en(2), nada))
			return i.NoReejecutar("test", evidencia, previo, en(3), nada)
		},
		"comienzo después de una no re-ejecución": func(i *dominio.Intento) error {
			require.NoError(t, i.NoReejecutar("test", evidencia, previo, en(2), nada))
			return i.Comenzar("test", en(3), nada)
		},
	}
	for nombre, registrar := range casos {
		t.Run(nombre, func(t *testing.T) {
			i := abierto(t, "i1", aperturaEn("staging"))
			require.ErrorIs(t, registrar(i), dominio.ErrRechazado)
		})
	}
}

func TestIntento_LaEvidenciaApuntaAUnFinalExitoso(t *testing.T) {
	bueno := abierto(t, "bueno", aperturaEn("staging"))
	hacer(t, bueno, 1, "test")

	fallido := abierto(t, "fallido", aperturaEn("staging"))
	require.NoError(t, fallido.Comenzar("test", en(1), nada))
	require.NoError(t, fallido.Terminar("test", false, en(2), nada))

	sinFinal := abierto(t, "sin-final", aperturaEn("staging"))
	require.NoError(t, sinFinal.Comenzar("test", en(1), nada))

	casos := map[string]struct {
		evidencia dominio.Evidencia
		apuntado  *dominio.Intento
	}{
		"sin el intento apuntado":      {dominio.Evidencia{Intento: "bueno", Paso: "test"}, nil},
		"otro intento que el apuntado": {dominio.Evidencia{Intento: "bueno", Paso: "test"}, fallido},
		"un final fallido":             {dominio.Evidencia{Intento: "fallido", Paso: "test"}, fallido},
		"un comienzo sin final":        {dominio.Evidencia{Intento: "sin-final", Paso: "test"}, sinFinal},
		"un paso que no terminó ahí":   {dominio.Evidencia{Intento: "bueno", Paso: "deploy"}, bueno},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			i := abierto(t, "i1", aperturaEn("staging"))
			require.ErrorIs(t, i.NoReejecutar("test", c.evidencia, c.apuntado, en(3), nada), dominio.ErrRechazado)
		})
	}

	i := abierto(t, "i1", aperturaEn("staging"))
	require.NoError(t, i.NoReejecutar("test", dominio.Evidencia{Intento: "bueno", Paso: "test"}, bueno, en(3), nada))
}

func TestIntento_UnaVariableVaBajoUnPasoEnCurso(t *testing.T) {
	i := abierto(t, "i1", aperturaEn("staging"))
	require.ErrorIs(t, i.RegistrarVariable("test", "REPLICAS", en(1), nada), dominio.ErrRechazado, "antes del comienzo")

	require.NoError(t, i.Comenzar("test", en(1), nada))
	require.ErrorIs(t, i.RegistrarVariable("test", "", en(2), nada), dominio.ErrRechazado, "sin nombre")
	require.NoError(t, i.RegistrarVariable("test", "REPLICAS", en(2), nada))
	require.NoError(t, i.GuardarValor("test", "REPLICAS", "3", en(2)))
	require.NoError(t, i.GuardarValor("test", "REPLICAS", "5", en(3)))
	require.NoError(t, i.Terminar("test", true, en(4), nada))

	require.ErrorIs(t, i.GuardarValor("test", "REPLICAS", "7", en(5)), dominio.ErrRechazado, "después del final")
	require.Equal(t, map[dominio.NombreVariable]string{"REPLICAS": "5"}, i.Valores("test"))
	require.Len(t, i.Variables("test"), 1)
}

func TestIntento_NingunRegistroDespuesDelCierreODelAbandono(t *testing.T) {
	terminaciones := map[string]func(*dominio.Intento){
		"cierre":   func(i *dominio.Intento) { cerrar(t, i, dominio.Fallido) },
		"abandono": func(i *dominio.Intento) { require.NoError(t, i.Abandonar(en(9))) },
	}
	for nombre, terminar := range terminaciones {
		t.Run(nombre, func(t *testing.T) {
			i := abierto(t, "i1", aperturaEn("staging"))
			require.NoError(t, i.Comenzar("test", en(1), nada))
			terminar(i)

			require.ErrorIs(t, i.Terminar("test", true, en(10), nada), dominio.ErrRechazado)
			require.ErrorIs(t, i.Comenzar("supply", en(10), nada), dominio.ErrRechazado)
			require.ErrorIs(t, i.Abandonar(en(10)), dominio.ErrRechazado)
			require.ErrorIs(t,
				i.Cerrar(dominio.Cierre{Estado: dominio.Exitoso}, en(10), dominio.NuevosDesplieguesDeUnAmbiente("staging")),
				dominio.ErrRechazado)
		})
	}
}

func TestIntento_SinDesenlaceYAbandonado(t *testing.T) {
	require.False(t, dominio.NuevoIntento("vacio").SinDesenlace(), "sin registros no hay intento")

	i := abierto(t, "i1", aperturaEn("staging"))
	require.True(t, i.SinDesenlace())
	require.False(t, i.Terminado())

	require.NoError(t, i.Abandonar(en(1)))
	require.True(t, i.SinDesenlace(), "abandonado no es un estado")
	require.True(t, i.Abandonado())
	require.True(t, i.Terminado())

	cerrado := abierto(t, "i2", aperturaEn("staging"))
	cerrar(t, cerrado, dominio.Cancelado)
	cierre, ok := cerrado.Cierre()
	require.True(t, ok)
	require.Equal(t, dominio.Cancelado, cierre.Estado)
	require.False(t, cerrado.SinDesenlace())
}

func TestIntento_UnIntentoSinAperturaSeAbandona(t *testing.T) {
	i := dominio.NuevoIntento("i1")
	require.NoError(t, i.Abandonar(en(1)))
	require.True(t, i.Terminado())
}

func TestIntento_ElCierreExitosoExigeLosPasosPedidos(t *testing.T) {
	i := abierto(t, "i1", aperturaEn("staging"))
	hacer(t, i, 1, "test", "supply")
	require.NoError(t, i.Comenzar("deploy", en(2), nada))

	despliegues := dominio.NuevosDesplieguesDeUnAmbiente("staging")
	require.ErrorIs(t, i.Cerrar(dominio.Cierre{Estado: dominio.Exitoso}, en(3), despliegues), dominio.ErrRechazado)
	require.NoError(t, i.Cerrar(dominio.Cierre{Estado: dominio.Fallido}, en(3), despliegues))
}

func TestIntento_ElDestinoEsUnDespliegueDeSuAmbiente(t *testing.T) {
	i := abierto(t, "i1", aperturaEn("staging"))
	hacer(t, i, 1, "test", "supply", "deploy")

	despliegues := dominio.NuevosDesplieguesDeUnAmbiente("staging")
	rollback := dominio.Cierre{Estado: dominio.Exitoso, Destino: "d-inexistente"}
	require.ErrorIs(t, i.Cerrar(rollback, en(2), despliegues), dominio.ErrRechazado)
	require.ErrorIs(t, i.Cerrar(dominio.Cierre{Estado: dominio.Exitoso}, en(2),
		dominio.NuevosDesplieguesDeUnAmbiente("prod")), dominio.ErrRechazado, "despliegues de otro ambiente")
	require.True(t, i.SinDesenlace(), "un cierre rechazado no se escribe")
}

func TestReconstituirIntento_CompruebaCadaRegistro(t *testing.T) {
	i := abierto(t, "i1", aperturaEn("staging"))
	hacer(t, i, 1, "test")

	leido, err := dominio.ReconstituirIntento("i1", i.Registros())
	require.NoError(t, err)
	require.Equal(t, 3, leido.Leidos())
	require.Empty(t, leido.Nuevos())

	require.NoError(t, leido.Comenzar("supply", en(2), nada))
	require.Len(t, leido.Nuevos(), 1)

	desordenados := i.Registros()
	desordenados[1], desordenados[2] = desordenados[2], desordenados[1]
	_, err = dominio.ReconstituirIntento("i1", desordenados)
	require.ErrorIs(t, err, dominio.ErrRechazado, "un final antes de su comienzo")
}

func TestUltimaVezDeUnPaso_EsElUltimoRegistroEnSuAmbito(t *testing.T) {
	primero := abierto(t, "s1", aperturaEn("staging"))
	hacer(t, primero, 10, "test", "supply", "deploy")

	enProd := abierto(t, "p1", aperturaEn("prod"))
	hacer(t, enProd, 20, "test", "supply")

	segundo := abierto(t, "s2", aperturaEn("staging"))
	hacer(t, segundo, 15, "test")
	require.NoError(t, segundo.Comenzar("supply", en(16), nada))

	otroAmbito := aperturaEn("staging")
	otroAmbito.Pasos[2].Compartido = true
	distinto := abierto(t, "s3", otroAmbito)
	hacer(t, distinto, 30, "test", "supply", "deploy")

	staging := []*dominio.Intento{primero, segundo, distinto}
	todos := []*dominio.Intento{primero, enProd, segundo, distinto}

	t.Run("en su ambiente gana el último del orden", func(t *testing.T) {
		r, de, ok := dominio.UltimaVezDeUnPaso(
			dominio.PasoEnSuAmbito{Paso: "deploy", Ambito: dominio.AmbitoDeAmbiente("staging")}, staging)
		require.True(t, ok)
		require.Equal(t, dominio.IdIntento("s1"), de, "s3 declaró deploy compartido: es otra posición")
		require.Equal(t, dominio.TipoFinal, r.Tipo)
	})

	t.Run("un comienzo sin final también es la última vez", func(t *testing.T) {
		r, de, ok := dominio.UltimaVezDeUnPaso(
			dominio.PasoEnSuAmbito{Paso: "supply", Ambito: dominio.AmbitoCompartido()}, staging[:2])
		require.True(t, ok)
		require.Equal(t, dominio.IdIntento("s2"), de)
		require.Equal(t, dominio.TipoComienzo, r.Tipo)
	})

	t.Run("en el compartido gana el instante mayor de cualquier ambiente", func(t *testing.T) {
		r, de, ok := dominio.UltimaVezDeUnPaso(
			dominio.PasoEnSuAmbito{Paso: "supply", Ambito: dominio.AmbitoCompartido()}, todos)
		require.True(t, ok)
		require.Equal(t, dominio.IdIntento("s3"), de)
		require.Equal(t, en(30), r.Instante)

		_, de, ok = dominio.UltimaVezDeUnPaso(
			dominio.PasoEnSuAmbito{Paso: "supply", Ambito: dominio.AmbitoCompartido()}, todos[:3])
		require.True(t, ok)
		require.Equal(t, dominio.IdIntento("p1"), de, "prod en el minuto 20 va después del comienzo de s2 en el 16")
	})

	t.Run("sin registros no hay última vez", func(t *testing.T) {
		_, _, ok := dominio.UltimaVezDeUnPaso(
			dominio.PasoEnSuAmbito{Paso: "deploy", Ambito: dominio.AmbitoDeAmbiente("prod")}, todos)
		require.False(t, ok)
	})
}
