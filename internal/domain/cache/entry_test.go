package cache_test

// El TTL como METADATO de la entrada, no como material de la clave (spec 10
// §5.1). El tiempo no es propiedad del contenido: una entrada caducada manda
// ejecutar, y la entrada nueva ocupa el mismo sitio bajo la MISMA clave.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
)

var instante = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

func TestEntry_Caducidad(t *testing.T) {
	procedencia := cache.Provenance{ExecutionID: "exec-1", At: instante}
	entrada := cache.NewEntry(procedencia, cache.DefaultTTL)

	t.Run("dentro del TTL no ha caducado", func(t *testing.T) {
		assert.False(t, entrada.IsExpired(instante.Add(29*24*time.Hour)))
	})

	t.Run("el borde es exclusivo: caduca AL alcanzarlo", func(t *testing.T) {
		require.NotNil(t, entrada.ExpiresAt)
		assert.True(t, entrada.IsExpired(*entrada.ExpiresAt))
		assert.False(t, entrada.IsExpired(entrada.ExpiresAt.Add(-time.Nanosecond)))
	})

	t.Run("pasado el TTL ha caducado", func(t *testing.T) {
		assert.True(t, entrada.IsExpired(instante.Add(31*24*time.Hour)))
	})

	t.Run("un TTL no positivo es una entrada que no caduca", func(t *testing.T) {
		perpetua := cache.NewEntry(procedencia, 0)
		assert.Nil(t, perpetua.ExpiresAt)
		assert.False(t, perpetua.IsExpired(instante.Add(100*365*24*time.Hour)))
	})
}

// Caducar NO cambia la clave: es lo que distingue el TTL como metadato del TTL
// como material. Si el tiempo entrara en la clave, cada re-ejecución por
// caducidad dejaría una entrada nueva y la vieja quedaría huérfana para siempre.
func TestEntry_CaducarNoCambiaLaClave(t *testing.T) {
	material := materialBase(t)

	primera, err := cache.NewCacheKey(material)
	require.NoError(t, err)

	// Pasan 40 días y el paso se re-ejecuta con el mismo contenido.
	segunda, err := cache.NewCacheKey(material)
	require.NoError(t, err)

	assert.True(t, primera.Equals(segunda),
		"la entrada nueva sustituye a la vieja en su sitio")
}

// La procedencia es lo que hace respondible «¿cuándo se probó esto por última
// vez?» cuando un paso se salta con una clave opaca (spec 10 §5.4).
func TestEntry_GuardaQuienLaEscribioYCuando(t *testing.T) {
	entrada := cache.NewEntry(
		cache.Provenance{ExecutionID: "exec-42", At: instante}, cache.DefaultTTL)

	assert.Equal(t, "exec-42", entrada.ProducedBy.ExecutionID)
	assert.True(t, instante.Equal(entrada.ProducedBy.At))
}
