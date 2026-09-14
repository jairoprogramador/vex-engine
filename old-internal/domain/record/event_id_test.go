package record_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

var entropiaFija = []byte{0xaa, 0xbb, 0xcc, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}

func TestEventID_EsUnUUIDv7(t *testing.T) {
	instante := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)

	id, err := record.NewEventID(instante, entropiaFija)
	require.NoError(t, err)

	texto := id.String()
	require.Len(t, texto, 36)
	assert.Equal(t, byte('7'), texto[14], "el nibble de versión de un UUIDv7 es 7")
	assert.Contains(t, "89ab", string(texto[19]), "los dos bits altos de la variante son 10")
	assert.False(t, id.IsZero())
}

func TestEventID_EsDeterministaConElMismoInstanteYLaMismaEntropia(t *testing.T) {
	// Es lo que permite fijar un event_id entero en un test: ni el reloj ni la
	// aleatoriedad viven dentro del dominio.
	instante := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)

	uno, err := record.NewEventID(instante, entropiaFija)
	require.NoError(t, err)
	otro, err := record.NewEventID(instante, entropiaFija)
	require.NoError(t, err)

	assert.True(t, uno.Equals(otro))
}

func TestEventID_OrdenaPorTiempo(t *testing.T) {
	antes, err := record.NewEventID(
		time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC), entropiaFija)
	require.NoError(t, err)
	despues, err := record.NewEventID(
		time.Date(2026, 8, 8, 10, 0, 1, 0, time.UTC), entropiaFija)
	require.NoError(t, err)

	assert.Less(t, antes.String(), despues.String())
}

func TestEventID_DosHechosIdenticosEnInstantesDistintosNoColisionan(t *testing.T) {
	// La identidad de un evento es CIRCUNSTANCIAL: no depende de lo que dice. Con
	// un hash del contenido, dos hechos iguales colapsarían en uno.
	uno, err := record.NewEventID(
		time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC), entropiaFija)
	require.NoError(t, err)
	otro, err := record.NewEventID(
		time.Date(2026, 8, 8, 10, 0, 0, 1_000_000, time.UTC), entropiaFija)
	require.NoError(t, err)

	assert.False(t, uno.Equals(otro))
}

func TestEventID_MaterialInvalido(t *testing.T) {
	t.Run("sin instante", func(t *testing.T) {
		_, err := record.NewEventID(time.Time{}, entropiaFija)
		require.Error(t, err)
	})

	t.Run("con la entropía de otro tamaño", func(t *testing.T) {
		_, err := record.NewEventID(time.Now(), []byte{0x01, 0x02})
		require.Error(t, err)
	})

	t.Run("con un instante que no cabe en 48 bits", func(t *testing.T) {
		_, err := record.NewEventID(time.Date(1969, 1, 1, 0, 0, 0, 0, time.UTC), entropiaFija)
		require.Error(t, err)

		_, err = record.NewEventID(time.Date(300000, 1, 1, 0, 0, 0, 0, time.UTC), entropiaFija)
		require.Error(t, err)
	})
}

func TestEventID_SeLeeYSeEscribeIgual(t *testing.T) {
	id, err := record.NewEventID(time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC), entropiaFija)
	require.NoError(t, err)

	leido, err := record.ParseEventID(id.String())
	require.NoError(t, err)
	assert.True(t, id.Equals(leido))
}

func TestEventID_LoQueNoEsUnUUIDEsUnError(t *testing.T) {
	for _, texto := range []string{"", "no-es-un-uuid", "0189b1c2"} {
		_, err := record.ParseEventID(texto)
		require.Error(t, err, "se esperaba error para %q", texto)
	}
}

func TestEventID_ElValorCeroNoEsUnaIdentidad(t *testing.T) {
	var id record.EventID
	assert.True(t, id.IsZero())
	assert.Empty(t, id.String())
}
