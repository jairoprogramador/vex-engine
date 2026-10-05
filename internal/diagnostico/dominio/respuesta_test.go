package dominio_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/diagnostico/dominio"
)

func TestRespuesta(t *testing.T) {
	t.Run("con atribución da su forma, la atribución y el sustento", func(t *testing.T) {
		atribucion := dominio.NuevaAtribucion([]dominio.Eje{dominio.Variables})
		sustento := dominio.NuevoSustento(dominio.IdIntento{}, time.Time{}, nil)
		r := dominio.NuevaRespuestaConAtribucion(atribucion, sustento)

		require.Equal(t, dominio.ConAtribucion, r.Forma())
		a, s, ok := r.AtribucionConSustento()
		require.True(t, ok)
		require.Equal(t, atribucion, a)
		require.Equal(t, sustento, s)
	})

	t.Run("sin referencia no da atribución", func(t *testing.T) {
		r := dominio.RespuestaSinReferencia()
		require.Equal(t, dominio.SinReferencia, r.Forma())
		require.Equal(t, dominio.MensajeSinHistorialPrevio, r.Mensaje())
		_, _, ok := r.AtribucionConSustento()
		require.False(t, ok)
	})

	t.Run("no se atribuye no da atribución", func(t *testing.T) {
		r := dominio.RespuestaNoSeAtribuye()
		require.Equal(t, dominio.NoSeAtribuye, r.Forma())
		require.Empty(t, r.Mensaje())
		_, _, ok := r.AtribucionConSustento()
		require.False(t, ok)
	})
}
