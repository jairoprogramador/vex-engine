package state_test

// El ULID y la única propiedad que se le pide: que su orden LEXICOGRÁFICO sea el
// TEMPORAL. De ahí sale «el último registro se obtiene sin leer ninguno».

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/state"
)

func TestRecordID_ElOrdenLexicograficoEsElTemporal(t *testing.T) {
	entropiaAlta := []byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255}
	entropiaBaja := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}

	// El posterior lleva la entropía MÍNIMA y el anterior la máxima: si el orden
	// dependiera de la parte aleatoria, este caso saldría al revés.
	anterior, err := state.NewRecordID(instante, entropiaAlta)
	require.NoError(t, err)
	posterior, err := state.NewRecordID(instante.Add(time.Millisecond), entropiaBaja)
	require.NoError(t, err)

	assert.Less(t, anterior.String(), posterior.String())
}

func TestRecordID_MismoInstanteYDistintaEntropiaSonDistintos(t *testing.T) {
	uno, err := state.NewRecordID(instante, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)
	otro, err := state.NewRecordID(instante, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 11})
	require.NoError(t, err)

	assert.NotEqual(t, uno.String(), otro.String(),
		"dos ejecuciones del mismo milisegundo son dos hechos distintos")
}

func TestRecordID_FormaCanonica(t *testing.T) {
	id, err := state.NewRecordID(instante, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	require.NoError(t, err)

	assert.Len(t, id.String(), state.RecordIDLen)
	assert.Regexp(t, `^[0-9A-HJKMNP-TV-Z]{26}$`, id.String(),
		"alfabeto de Crockford: sin I, L, O ni U")

	roundTrip, err := state.ParseRecordID(id.String())
	require.NoError(t, err)
	assert.Equal(t, id.String(), roundTrip.String())
}

// El vector fija la codificación entera —los 48 bits de instante y los 80 de
// entropía— para que un cambio en el empaquetado de bits no pase inadvertido.
//
// Los diez primeros símbolos son 1786017600000 (el instante en milisegundos) en
// base32 de Crockford; los dieciséis siguientes, los bytes 00…09 de entropía.
// Las dos mitades se comprueban por separado abajo, así que un fallo aquí dice
// cuál de las dos se movió.
func TestRecordID_Vector(t *testing.T) {
	momento := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	require.Equal(t, int64(1786017600000), momento.UnixMilli())

	id, err := state.NewRecordID(momento, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.NoError(t, err)

	assert.Equal(t, "01KZBF3MG0", id.String()[:10], "los 48 bits del instante")
	assert.Equal(t, "000G40R40M30E209", id.String()[10:], "los 80 bits de entropía")
	assert.Equal(t, "01KZBF3MG0000G40R40M30E209", id.String())
}

func TestRecordID_SeRechazaLoQueNoEsOrdenable(t *testing.T) {
	t.Run("sin instante", func(t *testing.T) {
		_, err := state.NewRecordID(time.Time{}, make([]byte, 10))
		assert.Error(t, err)
	})

	t.Run("entropía de otro tamaño", func(t *testing.T) {
		_, err := state.NewRecordID(instante, make([]byte, 9))
		assert.Error(t, err)
	})

	t.Run("parse de una forma que no es un ULID", func(t *testing.T) {
		for _, texto := range []string{"", "corto", "01J9X3QK8F7ZB0XKQ2M4V6T8HNX", "01J9X3QK8F7ZB0XKQ2M4V6T8HI"} {
			_, err := state.ParseRecordID(texto)
			assert.Error(t, err, texto)
		}
	})
}
