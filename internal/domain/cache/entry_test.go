package cache_test

// La entrada del ÍNDICE, tras el cambio de papel de la spec 11: un puntero de
// contenido a registro, sin caducidad y sin procedencia.
//
// Aquí vivían los tests del TTL. No se han movido: han desaparecido con lo que
// probaban. La expiración es una propiedad del REGISTRO —de su edad— y no del
// índice, que ya no decide nada; la declara el pipelinecode con la spec 15.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

func TestEntry_ApuntaAUnRegistroConcreto(t *testing.T) {
	entrada, err := cache.NewEntry(claveDeEstado(t), recordIDDePrueba(t))
	require.NoError(t, err)

	assert.Equal(t, "02-supply", entrada.StateKey.StepID())
	assert.Equal(t, "environment:sand", entrada.StateKey.Scope().String())
	assert.False(t, entrada.RecordID.IsZero())
}

// Un puntero incompleto es un error: apuntar a ninguna parte es peor que no
// apuntar, porque un índice con entradas rotas deja de ser reconstruible sin
// distinguir cuáles lo están.
func TestEntry_UnPunteroIncompletoEsUnError(t *testing.T) {
	t.Run("sin clave de estado", func(t *testing.T) {
		_, err := cache.NewEntry(state.Key{}, recordIDDePrueba(t))
		assert.Error(t, err)
	})

	t.Run("sin registro", func(t *testing.T) {
		_, err := cache.NewEntry(claveDeEstado(t), state.RecordID{})
		assert.Error(t, err)
	})
}

// La clave de contenido no depende del tiempo: dos ejecuciones del mismo
// material dan la misma clave, y lo que cambia es a qué registro apunta.
func TestEntry_ElMismoMaterialDaLaMismaClave(t *testing.T) {
	material := materialBase(t)

	primera, err := cache.NewCacheKey(material)
	require.NoError(t, err)
	segunda, err := cache.NewCacheKey(material)
	require.NoError(t, err)

	assert.True(t, primera.Equals(segunda))
}

func claveDeEstado(t *testing.T) state.Key {
	t.Helper()
	scope, err := state.NewEnvironmentScope("sand")
	require.NoError(t, err)
	key, err := state.NewKey("https://vex.test/acme/demo-app.git", scope, "02-supply")
	require.NoError(t, err)
	return key
}

func recordIDDePrueba(t *testing.T) state.RecordID {
	t.Helper()
	id, err := state.NewRecordID(
		time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC),
		[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.NoError(t, err)
	return id
}
