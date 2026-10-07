package dominio_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

func campoYValor(err error) (campo, valor string, ok bool) {
	var parametro interface{ ParametroInvalido() (string, string) }
	if !errors.As(err, &parametro) {
		return "", "", false
	}
	campo, valor = parametro.ParametroInvalido()
	return campo, valor, true
}

func TestParametroInvalido_LosValoresDeLaPeticionDicenSuCampoYSuValor(t *testing.T) {
	pipeline := dominio.Pipeline{
		Ambientes: []dominio.Ambiente{{Nombre: "sandbox", Valor: "sand"}},
		Pasos:     []dominio.Paso{{Nombre: "test"}},
	}
	casos := map[string]struct {
		err          func() error
		campo, valor string
	}{
		"ámbito de ambiente vacío":            {func() error { _, err := dominio.AmbitoDeAmbiente(""); return err }, "Ambiente", ""},
		"ambiente que el pipeline no declara": {func() error { _, err := pipeline.AmbientePorValor("prod"); return err }, "Ambiente", "prod"},
		"hasta un paso que no existe":         {func() error { _, err := pipeline.PasosHasta("deploy"); return err }, "HastaPaso", "deploy"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			err := c.err()

			require.ErrorIs(t, err, dominio.ErrInvalido)
			campo, valor, ok := campoYValor(err)
			require.True(t, ok)
			require.Equal(t, c.campo, campo)
			require.Equal(t, c.valor, valor)
		})
	}
}

func TestParametroInvalido_UnaExpresionRegularMalaDelPipelineNoInventaUnCampo(t *testing.T) {
	_, err := dominio.FabricarSalidaSimulada("(sin cerrar")

	require.ErrorIs(t, err, dominio.ErrInvalido)
	_, _, ok := campoYValor(err)
	require.False(t, ok, "es del pipeline, no un campo de la petición")
}
