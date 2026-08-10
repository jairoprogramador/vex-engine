package cache_test

// Contrato del índice, con la disciplina del contract test de la spec 02.
//
// Lo que cambia respecto de la spec 10 no son los casos sino lo que significan:
// esto ya no decide si un step se ejecuta, así que su política de error se
// invierte —ilegible ⇒ ausente— y sus escrituras dejan de ser una afirmación
// sobre el mundo para ser un puntero a donde vive esa afirmación.

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
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
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

func claveDeEstado(t *testing.T, step string) domState.Key {
	t.Helper()
	scope, err := domState.NewEnvironmentScope("sand")
	require.NoError(t, err)
	key, err := domState.NewKey("https://vex.test/acme/demo-app", scope, step)
	require.NoError(t, err)
	return key
}

func entradaDePrueba(t *testing.T, step string, at time.Time) domCache.Entry {
	t.Helper()
	recordID, err := domState.NewRecordID(at, []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.NoError(t, err)
	entry, err := domCache.NewEntry(claveDeEstado(t, step), recordID)
	require.NoError(t, err)
	return entry
}

func TestFileEntriesRepository_Contrato(t *testing.T) {
	ctx := context.Background()

	t.Run("una clave que nadie escribió NO consta, y no es un error", func(t *testing.T) {
		repo := infraCache.NewFileEntriesRepository(t.TempDir())

		_, existe, err := repo.Get(&ctx, claveDePrueba(t, "supply"))

		require.NoError(t, err)
		assert.False(t, existe)
	})

	t.Run("lo escrito se lee igual", func(t *testing.T) {
		repo := infraCache.NewFileEntriesRepository(t.TempDir())
		key := claveDePrueba(t, "supply")
		escrita := entradaDePrueba(t, "02-supply", instante)

		require.NoError(t, repo.Put(&ctx, key, escrita))

		leida, existe, err := repo.Get(&ctx, key)
		require.NoError(t, err)
		require.True(t, existe)

		assert.True(t, escrita.StateKey.Equals(leida.StateKey),
			"la clave de estado va y vuelve entera: subject, ámbito y step")
		assert.Equal(t, escrita.RecordID.String(), leida.RecordID.String())
	})

	t.Run("consultar no crea la entrada", func(t *testing.T) {
		// La red de regresión de la spec 09 §9.10: si `Get` escribiera, una
		// muerte dura entre la consulta y el final del step dejaría grabado «ya se
		// hizo» para un step que nunca terminó.
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
		// La propiedad que sobrevive del direccionamiento por contenido: el
		// índice conserva la respuesta de A cuando se escribe la de B. Ya no
		// produce un acierto de caché —eso lo decide el último registro— pero es
		// lo que haría barato revertir esa regresión (spec 11 §5.5).
		base := t.TempDir()
		repo := infraCache.NewFileEntriesRepository(base)

		supply := claveDePrueba(t, "supply")
		deploy := claveDePrueba(t, "deploy")

		require.NoError(t, repo.Put(&ctx, supply, entradaDePrueba(t, "02-supply", instante)))
		require.NoError(t, repo.Put(&ctx, deploy, entradaDePrueba(t, "04-deploy", instante)))

		_, existeSupply, err := repo.Get(&ctx, supply)
		require.NoError(t, err)
		_, existeDeploy, err := repo.Get(&ctx, deploy)
		require.NoError(t, err)

		assert.True(t, existeSupply)
		assert.True(t, existeDeploy)
		assert.Len(t, archivosDe(t, base), 2)
	})

	t.Run("reescribir la misma clave la actualiza en su sitio", func(t *testing.T) {
		// El índice NO es append-only: es derivable, y lo que guarda es «el
		// registro más reciente de este contenido». Que se sustituya en su sitio
		// es lo correcto, y es la diferencia visible con el almacén de registros.
		base := t.TempDir()
		repo := infraCache.NewFileEntriesRepository(base)
		key := claveDePrueba(t, "supply")

		require.NoError(t, repo.Put(&ctx, key, entradaDePrueba(t, "02-supply", instante)))
		masTarde := entradaDePrueba(t, "02-supply", instante.Add(24*time.Hour))
		require.NoError(t, repo.Put(&ctx, key, masTarde))

		leida, _, err := repo.Get(&ctx, key)
		require.NoError(t, err)
		assert.Equal(t, masTarde.RecordID.String(), leida.RecordID.String())
		assert.Len(t, archivosDe(t, base), 1, "una entrada por clave, no un historial")
	})

	t.Run("con dos máquinas compartiendo destino gana el registro más reciente", func(t *testing.T) {
		// El desempate que la spec 21 §8 tenía abierto y cierra: desde la spec 16
		// el índice cuelga del destino, así que dos máquinas pueden escribir la
		// MISMA clave apuntando a dos registros distintos y ambos válidos. Gana el
		// `record_id` mayor —que por ser un ULID es el más reciente— y no «el
		// último que escriba», que depende del orden de llegada y no de los hechos.
		base := t.TempDir()
		repo := infraCache.NewFileEntriesRepository(base)
		key := claveDePrueba(t, "supply")

		reciente := entradaDePrueba(t, "02-supply", instante.Add(24*time.Hour))
		anterior := entradaDePrueba(t, "02-supply", instante)

		require.NoError(t, repo.Put(&ctx, key, reciente))
		require.NoError(t, repo.Put(&ctx, key, anterior))

		leida, _, err := repo.Get(&ctx, key)
		require.NoError(t, err)
		assert.Equal(t, reciente.RecordID.String(), leida.RecordID.String(),
			"escribir uno anterior no hace retroceder el índice")
	})

	t.Run("la clave vacía se rechaza en los dos sentidos", func(t *testing.T) {
		repo := infraCache.NewFileEntriesRepository(t.TempDir())
		var cero domCache.CacheKey

		_, _, err := repo.Get(&ctx, cero)
		assert.Error(t, err)
		assert.Error(t, repo.Put(&ctx, cero, entradaDePrueba(t, "02-supply", instante)))
	})
}

// El archivo es inspeccionable y se identifica a sí mismo. Es el argumento por
// el que la spec 10 descartó extender los `.status` gob que había: un gob no se
// puede abrir cuando el motor se salta un step y no se entiende por qué.
func TestFileEntriesRepository_ElArchivoSeExplicaSolo(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	repo := infraCache.NewFileEntriesRepository(base)
	key := claveDePrueba(t, "supply")
	entrada := entradaDePrueba(t, "02-supply", instante)

	require.NoError(t, repo.Put(&ctx, key, entrada))

	archivos := archivosDe(t, base)
	require.Len(t, archivos, 1)

	datos, err := os.ReadFile(archivos[0])
	require.NoError(t, err)

	var dto infraCache.FileCacheEntryDTO
	require.NoError(t, json.Unmarshal(datos, &dto))

	assert.Equal(t, key.String(), dto.CacheKey, "el archivo dice de qué clave es")
	assert.Equal(t, "environment:sand", dto.StateKey.Scope, "y a qué posición apunta")
	assert.Equal(t, "02-supply", dto.StateKey.StepID)
	assert.Equal(t, entrada.RecordID.String(), dto.RecordID)

	// La ruta está direccionada por contenido: la versión de la regla, un cajón
	// por los dos primeros caracteres del hash, y el resto.
	rel, err := filepath.Rel(base, archivos[0])
	require.NoError(t, err)
	assert.Equal(t,
		filepath.Join(key.Version(), key.Hash()[:2], key.Hash()[2:]+".json"),
		rel)
}

// ILEGIBLE ⇒ AUSENTE, que es LO CONTRARIO de lo que este mismo test afirmaba
// hasta la spec 10 — y el diff conviene mirarlo.
//
// Entonces la entrada decidía si un step se ejecutaba, así que leer una rota
// como «no consta» habría sido fail-open silencioso sobre una decisión real.
// Desde la spec 11 el índice no decide nada y es derivable: una entrada rota es
// exactamente una entrada que todavía no se ha reconstruido. La asimetría
// contraria vive en el almacén de registros, donde ilegible sí es error.
func TestFileEntriesRepository_UnArchivoIlegibleEsUnaAusencia(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	repo := infraCache.NewFileEntriesRepository(base)
	key := claveDePrueba(t, "supply")

	require.NoError(t, repo.Put(&ctx, key, entradaDePrueba(t, "02-supply", instante)))
	archivos := archivosDe(t, base)
	require.NoError(t, os.WriteFile(archivos[0], []byte("{no soy json"), 0o644))

	_, existe, err := repo.Get(&ctx, key)

	require.NoError(t, err)
	assert.False(t, existe)
}

// Una entrada del esquema viejo —presencia + caducidad + procedencia— no apunta
// a ningún registro, así que no se puede interpretar. Se ignora, que es lo que
// «derivable» significa en la práctica: se reescribirá sola.
func TestFileEntriesRepository_UnaEntradaDelEsquemaViejoSeIgnora(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	repo := infraCache.NewFileEntriesRepository(base)
	key := claveDePrueba(t, "supply")

	require.NoError(t, repo.Put(&ctx, key, entradaDePrueba(t, "02-supply", instante)))
	archivos := archivosDe(t, base)
	require.NoError(t, os.WriteFile(archivos[0], []byte(`{
  "schema_version": 1,
  "cache_key": "`+key.String()+`",
  "expires_at": "2026-09-05T12:00:00Z",
  "produced_by": {"execution_id": "exec-1", "at": "2026-08-06T12:00:00Z"}
}`), 0o644))

	_, existe, err := repo.Get(&ctx, key)

	require.NoError(t, err)
	assert.False(t, existe)
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
