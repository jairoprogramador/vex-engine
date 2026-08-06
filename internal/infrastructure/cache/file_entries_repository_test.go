package cache_test

// Contrato del almacén de entradas (spec 10 §5.2), con la disciplina del
// contract test de la spec 02.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domCache "github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
	infraCache "github.com/jairoprogramador/vex-engine/internal/infrastructure/cache"
)

var instante = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

func claveDePrueba(t *testing.T, step string) domCache.CacheKey {
	t.Helper()

	huella := func(version, digito string) fingerprint.Fingerprint {
		f, err := fingerprint.Parse(version + ":" + strings.Repeat(digito, 32))
		require.NoError(t, err)
		return f
	}

	key, err := domCache.NewCacheKey(domCache.Material{
		Subject:      "https://vex.test/acme/demo-app",
		Pipeline:     "https://vex.test/acme/pipelinecode",
		Scope:        "sand",
		Step:         step,
		Instructions: huella(fingerprint.InstructionsVersion, "11"),
		Variables:    huella(fingerprint.VariablesVersion, "22"),
		Code:         huella(fingerprint.Version, "33"),
	})
	require.NoError(t, err)
	return key
}

func entradaDePrueba() domCache.Entry {
	return domCache.NewEntry(
		domCache.Provenance{ExecutionID: "exec-1", At: instante}, domCache.DefaultTTL)
}

func TestFileEntriesRepository_Contrato(t *testing.T) {
	ctx := context.Background()

	t.Run("una clave que nadie escribió NO consta, y no es un error", func(t *testing.T) {
		// Es la mitad que sostiene todo lo demás: ausencia de entrada ⇒ ejecutar.
		// Si esto devolviera error, el motor confundiría «no consta» con «no pude
		// averiguarlo».
		repo := infraCache.NewFileEntriesRepository(t.TempDir())

		_, existe, err := repo.Get(&ctx, claveDePrueba(t, "supply"))

		require.NoError(t, err)
		assert.False(t, existe)
	})

	t.Run("lo escrito se lee igual", func(t *testing.T) {
		repo := infraCache.NewFileEntriesRepository(t.TempDir())
		key := claveDePrueba(t, "supply")
		escrita := entradaDePrueba()

		require.NoError(t, repo.Put(&ctx, key, escrita))

		leida, existe, err := repo.Get(&ctx, key)
		require.NoError(t, err)
		require.True(t, existe)

		assert.Equal(t, escrita.ProducedBy.ExecutionID, leida.ProducedBy.ExecutionID)
		assert.True(t, escrita.ProducedBy.At.Equal(leida.ProducedBy.At),
			"el instante va y vuelve sin perder precisión")
		require.NotNil(t, leida.ExpiresAt)
		assert.True(t, escrita.ExpiresAt.Equal(*leida.ExpiresAt))
	})

	t.Run("consultar no crea la entrada", func(t *testing.T) {
		// La red de regresión de la spec 09 §9.10 traducida a este modelo: si
		// `Get` escribiera, una muerte dura entre la consulta y el final del paso
		// dejaría grabado «ya se hizo» para un paso que nunca terminó.
		base := t.TempDir()
		repo := infraCache.NewFileEntriesRepository(base)
		key := claveDePrueba(t, "supply")

		for range 3 {
			_, existe, err := repo.Get(&ctx, key)
			require.NoError(t, err)
			require.False(t, existe)
		}

		assert.Empty(t, archivosDe(t, base), "consultar no dejó nada en disco")
	})

	t.Run("dos claves distintas conviven", func(t *testing.T) {
		// Es lo que hace que volver a un estado ya ejecutado acierte: la entrada
		// del estado A sigue en su sitio cuando se escribe la del B, en vez de ser
		// pisada por la última escritura (defecto (b) de la spec 10 §1).
		base := t.TempDir()
		repo := infraCache.NewFileEntriesRepository(base)

		supply := claveDePrueba(t, "supply")
		deploy := claveDePrueba(t, "deploy")

		require.NoError(t, repo.Put(&ctx, supply, entradaDePrueba()))
		require.NoError(t, repo.Put(&ctx, deploy, entradaDePrueba()))

		_, existeSupply, err := repo.Get(&ctx, supply)
		require.NoError(t, err)
		_, existeDeploy, err := repo.Get(&ctx, deploy)
		require.NoError(t, err)

		assert.True(t, existeSupply)
		assert.True(t, existeDeploy)
		assert.Len(t, archivosDe(t, base), 2)
	})

	t.Run("reescribir la misma clave la sustituye en su sitio", func(t *testing.T) {
		// Es lo que ocurre cuando una entrada caduca: la clave es idéntica.
		base := t.TempDir()
		repo := infraCache.NewFileEntriesRepository(base)
		key := claveDePrueba(t, "supply")

		require.NoError(t, repo.Put(&ctx, key, entradaDePrueba()))
		masTarde := domCache.NewEntry(
			domCache.Provenance{ExecutionID: "exec-2", At: instante.Add(40 * 24 * time.Hour)},
			domCache.DefaultTTL)
		require.NoError(t, repo.Put(&ctx, key, masTarde))

		leida, _, err := repo.Get(&ctx, key)
		require.NoError(t, err)
		assert.Equal(t, "exec-2", leida.ProducedBy.ExecutionID)
		assert.Len(t, archivosDe(t, base), 1, "una entrada por clave, no un historial")
	})

	t.Run("la clave vacía se rechaza en los dos sentidos", func(t *testing.T) {
		repo := infraCache.NewFileEntriesRepository(t.TempDir())
		var cero domCache.CacheKey

		_, _, err := repo.Get(&ctx, cero)
		assert.Error(t, err)
		assert.Error(t, repo.Put(&ctx, cero, entradaDePrueba()))
	})
}

// El archivo es inspeccionable y se identifica a sí mismo. Es el argumento por
// el que la spec 10 descartó extender los `.status` gob que había (alternativa
// D): un gob no se puede abrir cuando el motor se salta un paso y no se entiende
// por qué.
func TestFileEntriesRepository_ElArchivoSeExplicaSolo(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	repo := infraCache.NewFileEntriesRepository(base)
	key := claveDePrueba(t, "supply")

	require.NoError(t, repo.Put(&ctx, key, entradaDePrueba()))

	archivos := archivosDe(t, base)
	require.Len(t, archivos, 1)

	datos, err := os.ReadFile(archivos[0])
	require.NoError(t, err)

	var dto infraCache.FileCacheEntryDTO
	require.NoError(t, json.Unmarshal(datos, &dto))

	assert.Equal(t, key.String(), dto.CacheKey, "el archivo dice de qué clave es")
	assert.Equal(t, "exec-1", dto.ProducedBy.ExecutionID)
	assert.NotEmpty(t, dto.ExpiresAt)

	// La ruta está direccionada por contenido: la versión de la regla, un cajón
	// por los dos primeros caracteres del hash, y el resto.
	rel, err := filepath.Rel(base, archivos[0])
	require.NoError(t, err)
	assert.Equal(t,
		filepath.Join(key.Version(), key.Hash()[:2], key.Hash()[2:]+".json"),
		rel)
}

// Un archivo corrupto es un ERROR, no un «no consta». Tratarlo como ausencia
// sería fail-open silencioso; aguas arriba el paso se ejecuta igual, pero el
// usuario se entera de que su caché está roto (spec 09 §2).
func TestFileEntriesRepository_UnArchivoIlegibleEsUnError(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	repo := infraCache.NewFileEntriesRepository(base)
	key := claveDePrueba(t, "supply")

	require.NoError(t, repo.Put(&ctx, key, entradaDePrueba()))
	archivos := archivosDe(t, base)
	require.NoError(t, os.WriteFile(archivos[0], []byte("{no soy json"), 0o644))

	_, _, err := repo.Get(&ctx, key)

	assert.Error(t, err)
}

func archivosDe(t *testing.T, base string) []string {
	t.Helper()
	var encontrados []string
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			encontrados = append(encontrados, path)
		}
		return nil
	})
	require.NoError(t, err)
	return encontrados
}
