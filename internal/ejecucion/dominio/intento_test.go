package dominio_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func pasos(t *testing.T, nombres ...string) []dominio.PasoDelPipeline {
	t.Helper()
	var pasos []dominio.PasoDelPipeline
	for _, n := range nombres {
		p, err := dominio.NuevoPasoDelPipeline(n, false)
		require.NoError(t, err)
		pasos = append(pasos, p)
	}
	return pasos
}

func TestUnIntentoNoPuedeTenerElAmbienteVacio(t *testing.T) {
	_, err := dominio.NuevoIntentoEnCurso("", pasos(t, "01-pruebas"), "")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnIntentoNecesitaAlMenosUnPaso(t *testing.T) {
	_, err := dominio.NuevoIntentoEnCurso("prod", nil, "")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnIntentoRechazaUnHastaPasoQueNoEstaEnElPipeline(t *testing.T) {
	_, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas"), "no-existe")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestUnIntentoDaLosPasosEnOrden(t *testing.T) {
	intento, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas", "02-despliegue"), "")
	require.NoError(t, err)

	primero, ok := intento.SiguientePaso()
	require.True(t, ok)
	require.Equal(t, "01-pruebas", primero.Nombre())

	// Completar con el paso que no toca no avanza: la única forma de avanzar es completar el que toca.
	require.Error(t, intento.Completar("02-despliegue", true))
	segundo, ok := intento.SiguientePaso()
	require.True(t, ok)
	require.Equal(t, "01-pruebas", segundo.Nombre())

	require.NoError(t, intento.Completar("01-pruebas", true))
	tercero, ok := intento.SiguientePaso()
	require.True(t, ok)
	require.Equal(t, "02-despliegue", tercero.Nombre())
}

func TestUnIntentoNoDaNingunPasoDespuesDelPedido(t *testing.T) {
	intento, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas", "02-despliegue"), "01-pruebas")
	require.NoError(t, err)

	require.NoError(t, intento.Completar("01-pruebas", true))
	_, ok := intento.SiguientePaso()
	require.False(t, ok)

	desenlace, hay := intento.Desenlace()
	require.True(t, hay)
	require.Equal(t, dominio.Exitoso, desenlace)
}

func TestUnFalloCierraElIntentoYNoDaMasPasos(t *testing.T) {
	intento, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas", "02-despliegue"), "")
	require.NoError(t, err)

	require.NoError(t, intento.Completar("01-pruebas", false))

	_, ok := intento.SiguientePaso()
	require.False(t, ok)

	desenlace, hay := intento.Desenlace()
	require.True(t, hay)
	require.Equal(t, dominio.Fallido, desenlace)
}

func TestLaCancelacionGanaAUnFalloQueEllaMismaProvoca(t *testing.T) {
	intento, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas"), "")
	require.NoError(t, err)

	// El comando en curso se cancela y por eso falla: el paso se completa como no exitoso...
	require.NoError(t, intento.Completar("01-pruebas", false))
	// ...pero la cancelación, pedida justo después, sigue ganando el desenlace.
	intento.Cancelar()

	desenlace, hay := intento.Desenlace()
	require.True(t, hay)
	require.Equal(t, dominio.Cancelado, desenlace)
}

func TestCancelarEsIdempotente(t *testing.T) {
	intento, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas"), "")
	require.NoError(t, err)

	intento.Cancelar()
	intento.Cancelar()

	desenlace, hay := intento.Desenlace()
	require.True(t, hay)
	require.Equal(t, dominio.Cancelado, desenlace)
}

func TestUnIntentoSinTerminarNoTieneDesenlace(t *testing.T) {
	intento, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas", "02-despliegue"), "")
	require.NoError(t, err)

	_, hay := intento.Desenlace()
	require.False(t, hay)
}

func TestUnIntentoQueYaSeDetuvoNoAceptaMasPasos(t *testing.T) {
	intento, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas", "02-despliegue"), "")
	require.NoError(t, err)

	require.NoError(t, intento.Completar("01-pruebas", false))
	require.Error(t, intento.Completar("02-despliegue", true))
}

func TestUnDesenlaceExitosoEsEstableAlPreguntarloVariasVeces(t *testing.T) {
	intento, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas"), "")
	require.NoError(t, err)

	require.NoError(t, intento.Completar("01-pruebas", true))
	primero, hay := intento.Desenlace()
	require.True(t, hay)
	segundo, hay := intento.Desenlace()
	require.True(t, hay)

	require.Equal(t, dominio.Exitoso, primero)
	require.Equal(t, primero, segundo)
}
