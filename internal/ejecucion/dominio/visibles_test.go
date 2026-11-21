package dominio_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

func TestNuevaVariableVisible_UnValorNormalNoCambia(t *testing.T) {
	v := dominio.NuevaVariableVisible("url", "dónde quedó", "https://pagos.invalid")

	require.Equal(t, dominio.VariableVisible{Nombre: "url", Descripcion: "dónde quedó", Valor: "https://pagos.invalid"}, v)
}

func TestNuevaVariableVisible_RedactaElValor(t *testing.T) {
	v := dominio.NuevaVariableVisible("token", "", jwtDePrueba)

	require.Equal(t, dominio.Redactado, v.Valor)
}

func TestNuevaVariableVisible_CortaUnValorLargoSinPartirUnCaracter(t *testing.T) {
	justo := strings.Repeat("a", dominio.MaximoDeUnValorVisible)
	require.Equal(t, justo, dominio.NuevaVariableVisible("v", "", justo).Valor, "4 KiB justos caben")

	// La ñ ocupa dos bytes: con un byte delante, el corte a 4096 caería en medio de una.
	largo := "a" + strings.Repeat("ñ", dominio.MaximoDeUnValorVisible)
	valor := dominio.NuevaVariableVisible("v", "", largo).Valor

	require.True(t, utf8.ValidString(valor))
	require.True(t, strings.HasSuffix(valor, "… [TRUNCADO]"))
	require.LessOrEqual(t, len(valor), dominio.MaximoDeUnValorVisible+len("… [TRUNCADO]"))
}

func TestCaberEnLosVisibles(t *testing.T) {
	nuevas := make([]dominio.VariableVisible, 5)

	require.Len(t, dominio.CaberEnLosVisibles(0, nuevas), 5)
	require.Len(t, dominio.CaberEnLosVisibles(dominio.MaximoDeVisibles-5, nuevas), 5)
	require.Len(t, dominio.CaberEnLosVisibles(dominio.MaximoDeVisibles-2, nuevas), 2)
	require.Empty(t, dominio.CaberEnLosVisibles(dominio.MaximoDeVisibles, nuevas))
	require.Empty(t, dominio.CaberEnLosVisibles(dominio.MaximoDeVisibles+3, nuevas))
	require.Equal(t, 32, dominio.MaximoDeVisibles)
	require.Equal(t, 4096, dominio.MaximoDeUnValorVisible)
}
