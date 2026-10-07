package dominio_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// campoYValor son los que el error dice del parámetro que no vale; ok es false si no dice ninguno.
func campoYValor(err error) (campo, valor string, ok bool) {
	var parametro interface{ ParametroInvalido() (string, string) }
	if !errors.As(err, &parametro) {
		return "", "", false
	}
	campo, valor = parametro.ParametroInvalido()
	return campo, valor, true
}

func TestParametroInvalido_LosValoresDeLaPeticionDicenSuCampoYSuValor(t *testing.T) {
	pipeline := dominio.Pipeline{Ambientes: []string{"sand", "prod"}}
	casos := map[string]struct {
		err          func() error
		campo, valor string
	}{
		"ámbito de ambiente vacío": {func() error { _, err := dominio.AmbitoDeAmbiente(""); return err }, "Ambiente", ""},
		"intento sin ambiente": {func() error {
			_, err := dominio.NuevoIntentoEnCurso("", pasos(t, "01-pruebas"), "")
			return err
		}, "Ambiente", ""},
		"hasta un paso que no existe": {func() error {
			_, err := dominio.NuevoIntentoEnCurso("prod", pasos(t, "01-pruebas"), "no-existe")
			return err
		}, "HastaPaso", "no-existe"},
		"ambiente que el pipeline no declara": {func() error { return pipeline.ComprobarAmbiente("stag") }, "Ambiente", "stag"},
		"destino sin despliegue": {func() error {
			_, err := dominio.NuevoDestino("", "prod", "p", "c", "q", "d")
			return err
		}, "Despliegue", ""},
		"destino sin ambiente": {func() error {
			_, err := dominio.NuevoDestino("d1", "", "p", "c", "q", "d")
			return err
		}, "Ambiente", ""},
		"nombre de entorno inválido": {func() error {
			_, err := dominio.NuevoEntorno(map[string]string{"NOMBRE-MALO": "x"})
			return err
		}, "Entorno", "NOMBRE-MALO"},
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

func TestParametroInvalido_ElValorDeUnEntornoConNULNuncaSaleSoloSuNombre(t *testing.T) {
	_, err := dominio.NuevoEntorno(map[string]string{"TOKEN": "secreto\x00-123"})

	require.ErrorIs(t, err, dominio.ErrInvalido)
	campo, valor, ok := campoYValor(err)
	require.True(t, ok)
	require.Equal(t, "Entorno", campo)
	require.Equal(t, "TOKEN", valor, "el nombre de la variable, nunca su valor")
	require.False(t, strings.Contains(err.Error(), "secreto"), "ni en el texto")
}

func TestParametroInvalido_LasInvariantesInternasNoInventanUnCampo(t *testing.T) {
	_, errPaso := dominio.NuevoPasoDelPipeline("", false)
	_, errRegla := dominio.NuevaRegla(true, true, true, -1)
	_, errIntento := dominio.NuevoIntentoEnCurso("prod", nil, "")
	_, errDestino := dominio.NuevoDestino("d1", "prod", "", "", "q", "d")

	for nombre, err := range map[string]error{
		"paso del pipeline sin nombre": errPaso, "edad máxima negativa": errRegla,
		"intento sin pasos": errIntento, "destino sin fuente guardada": errDestino,
	} {
		t.Run(nombre, func(t *testing.T) {
			require.ErrorIs(t, err, dominio.ErrInvalido)
			_, _, ok := campoYValor(err)
			require.False(t, ok, "no es un campo de la petición")
		})
	}
}
