package dominio_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func hash(t *testing.T, valor string) dominio.HashDeCodigo {
	t.Helper()
	h, err := dominio.NuevoHashDeCodigo(valor)
	require.NoError(t, err)
	return h
}

func hashInstrucciones(t *testing.T, valor string) dominio.HashDeInstrucciones {
	t.Helper()
	h, err := dominio.NuevoHashDeInstrucciones(valor)
	require.NoError(t, err)
	return h
}

func reglaCon(t *testing.T, codigo, instrucciones, variables bool, edadMaxima time.Duration) dominio.Regla {
	t.Helper()
	r, err := dominio.NuevaRegla(codigo, instrucciones, variables, edadMaxima)
	require.NoError(t, err)
	return r
}

func TestDecidirPasoSinUltimaVezSiempreReejecuta(t *testing.T) {
	regla := reglaCon(t, true, true, true, 0)
	ahora := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)

	d := dominio.DecidirPaso(regla, ahora, dominio.UltimaVezDeUnPaso{Hay: false})

	require.True(t, d.SeReejecuta())
}

func TestDecidirPasoConComienzoSinFinalSiempreReejecuta(t *testing.T) {
	regla := reglaCon(t, true, true, true, 0)
	ahora := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)

	// Un comienzo sin final se traduce a Hay=true, Valida=false: no es un final exitoso.
	d := dominio.DecidirPaso(regla, ahora, dominio.UltimaVezDeUnPaso{Hay: true, Valida: false})

	require.True(t, d.SeReejecuta())
}

func TestDecidirPasoConFinalFallidoSiempreReejecuta(t *testing.T) {
	regla := reglaCon(t, true, true, true, 0)
	ahora := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)

	d := dominio.DecidirPaso(regla, ahora, dominio.UltimaVezDeUnPaso{Hay: true, Valida: false})

	require.True(t, d.SeReejecuta())
}

func TestDecidirPasoConFinalExitosoSinCambiosNoReejecuta(t *testing.T) {
	regla := reglaCon(t, true, true, true, time.Hour)
	recursos := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)

	d := dominio.DecidirPaso(regla, recursos, dominio.UltimaVezDeUnPaso{
		Hay: true, Valida: true, Recursos: recursos, Edad: time.Minute,
		Evidencia: dominio.Evidencia{Intento: "int-1", Paso: "01-pruebas"},
	})

	require.False(t, d.SeReejecuta())
	require.Equal(t, dominio.Evidencia{Intento: "int-1", Paso: "01-pruebas"}, d.Evidencia())
}

func TestDecidirPasoReejecutaSiCambioElCodigoYLaReglaLoMira(t *testing.T) {
	regla := reglaCon(t, true, false, false, 0)
	antes := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)
	ahora := dominio.NuevosRecursosDeUnPaso(hash(t, "c2"), hashInstrucciones(t, "i1"), false)

	d := dominio.DecidirPaso(regla, ahora, dominio.UltimaVezDeUnPaso{Hay: true, Valida: true, Recursos: antes})

	require.True(t, d.SeReejecuta())
}

func TestDecidirPasoNoReejecutaSiCambioElCodigoYLaReglaNoLoMira(t *testing.T) {
	regla := reglaCon(t, false, true, false, 0)
	antes := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)
	ahora := dominio.NuevosRecursosDeUnPaso(hash(t, "c2"), hashInstrucciones(t, "i1"), false)

	d := dominio.DecidirPaso(regla, ahora, dominio.UltimaVezDeUnPaso{Hay: true, Valida: true, Recursos: antes})

	require.False(t, d.SeReejecuta())
}

func TestDecidirPasoReejecutaSiCambiaronLasInstruccionesYLaReglaLasMira(t *testing.T) {
	regla := reglaCon(t, false, true, false, 0)
	antes := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)
	ahora := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i2"), false)

	d := dominio.DecidirPaso(regla, ahora, dominio.UltimaVezDeUnPaso{Hay: true, Valida: true, Recursos: antes})

	require.True(t, d.SeReejecuta())
}

func TestDecidirPasoReejecutaSiCambiaronLasVariablesYLaReglaLasMira(t *testing.T) {
	regla := reglaCon(t, false, false, true, 0)
	antes := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)
	ahora := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), true)

	d := dominio.DecidirPaso(regla, ahora, dominio.UltimaVezDeUnPaso{Hay: true, Valida: true, Recursos: antes})

	require.True(t, d.SeReejecuta())
}

func TestDecidirPasoNoReejecutaSiCambiaronLasVariablesYLaReglaNoLasMira(t *testing.T) {
	regla := reglaCon(t, true, true, false, 0)
	antes := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)
	ahora := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), true)

	d := dominio.DecidirPaso(regla, ahora, dominio.UltimaVezDeUnPaso{Hay: true, Valida: true, Recursos: antes})

	require.False(t, d.SeReejecuta())
}

func TestDecidirPasoReejecutaSiLaUltimaVezCaduco(t *testing.T) {
	regla := reglaCon(t, false, false, false, 30*time.Minute)
	recursos := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)

	d := dominio.DecidirPaso(regla, recursos, dominio.UltimaVezDeUnPaso{
		Hay: true, Valida: true, Recursos: recursos, Edad: 31 * time.Minute,
	})

	require.True(t, d.SeReejecuta())
}

func TestDecidirPasoNoReejecutaSiLaUltimaVezNoHaCaducado(t *testing.T) {
	regla := reglaCon(t, false, false, false, 30*time.Minute)
	recursos := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)

	d := dominio.DecidirPaso(regla, recursos, dominio.UltimaVezDeUnPaso{
		Hay: true, Valida: true, Recursos: recursos, Edad: 29 * time.Minute,
	})

	require.False(t, d.SeReejecuta())
}

func TestDecidirPasoConUnConjuntoDeReglasVacioSiempreReejecuta(t *testing.T) {
	regla := reglaCon(t, false, false, false, 0)
	recursos := dominio.NuevosRecursosDeUnPaso(hash(t, "c1"), hashInstrucciones(t, "i1"), false)

	d := dominio.DecidirPaso(regla, recursos, dominio.UltimaVezDeUnPaso{
		Hay: true, Valida: true, Recursos: recursos,
	})

	require.True(t, d.SeReejecuta())
}
