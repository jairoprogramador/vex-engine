package dominio_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

func TestUnaSenalCreceSiHayUnLatidoOUnRegistroNuevo(t *testing.T) {
	antes := dominio.SenalDeVida{Latidos: 2, Registros: 3}

	require.False(t, antes.CrecioDesde(antes))
	require.True(t, dominio.SenalDeVida{Latidos: 3, Registros: 3}.CrecioDesde(antes))
	require.True(t, dominio.SenalDeVida{Latidos: 2, Registros: 4}.CrecioDesde(antes))
}

func TestUnLatidoRecienteSaltaLaEsperaYUnoViejoNo(t *testing.T) {
	ahora := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	ventana := 15 * time.Second

	require.True(t, dominio.SenalDeVida{HayLatido: true, UltimoLatido: ahora.Add(-5 * time.Second)}.LatioDentroDe(ventana, ahora))
	require.False(t, dominio.SenalDeVida{HayLatido: true, UltimoLatido: ahora.Add(-time.Hour)}.LatioDentroDe(ventana, ahora))
	require.False(t, dominio.SenalDeVida{}.LatioDentroDe(ventana, ahora), "sin latidos no hay nada que mostrar")
}
