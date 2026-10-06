package dominio_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

func TestFabricarSalidaSimuladaCumpleLaExpresion(t *testing.T) {
	for _, expresion := range []string{
		"listo",
		"v[0-9]+\\.[0-9]+\\.[0-9]+",
		"^(sano|degradado)$",
		"[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}",
		"(.+)",
		"\\d{3,}",
		"^https?://[a-z]+\\.example\\.com/?$",
		"[^0-9]+",
		"a?b+c*",
		".",
	} {
		t.Run(expresion, func(t *testing.T) {
			salida, err := dominio.FabricarSalidaSimulada(expresion)
			require.NoError(t, err)
			require.Regexp(t, regexp.MustCompile(expresion), string(salida))
		})
	}
}

func TestFabricarSalidaSimuladaConExpresionInvalida(t *testing.T) {
	_, err := dominio.FabricarSalidaSimulada("(sin cerrar")
	require.ErrorIs(t, err, dominio.ErrInvalido)
}

func TestFabricarSalidaSimuladaNoDesbocaLasRepeticionesSinLimite(t *testing.T) {
	salida, err := dominio.FabricarSalidaSimulada("a+")
	require.NoError(t, err)
	require.LessOrEqual(t, len(salida), 10)
}
