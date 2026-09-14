package pipeline_test

// El repositorio de steps lee directorios y NADA más (spec 04 §5.2').
//
// Antes intentaba construir un `command.StepName` por directorio y descartaba en
// silencio el que no casaba: un typo como `4_test` hacía desaparecer un step del
// pipeline sin un mensaje. Ahora devuelve el nombre CRUDO de cada subdirectorio y
// es el validador de estructura quien decide —y quien nombra el culpable.
//
// Quién ordena y quién valida se prueba en internal/domain/pipeline.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	domPipeline "github.com/jairoprogramador/vex-engine/old-internal/domain/pipeline"
	infraPipeline "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pipelineConSteps(t *testing.T, dirNames ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range dirNames {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "steps", name), 0o755))
	}
	return root
}

func getEntries(t *testing.T, pipelinePath string) []string {
	t.Helper()
	ctx := context.Background()
	entries, err := infraPipeline.NewPipelineStepRepository().Get(&ctx, pipelinePath)
	require.NoError(t, err)

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.String())
	}
	return out
}

func TestPipelineStepRepository_Get_DevuelveLosNombresCrudos(t *testing.T) {
	t.Run("los directorios bien nombrados se devuelven tal cual", func(t *testing.T) {
		path := pipelineConSteps(t, "01-test", "02-supply", "03-package")

		assert.Equal(t, []string{"01-test", "02-supply", "03-package"}, getEntries(t, path))
	})

	t.Run("los directorios MAL nombrados también: ya no se descartan en silencio", func(t *testing.T) {
		// Este es el cambio de la spec 04 §1 (c): el repositorio no juzga, así que
		// el validador puede nombrar cada culpable en su error.
		path := pipelineConSteps(t, "01-test", "supply", "package-02", "-deploy", "4_test", "2-supply")

		assert.ElementsMatch(t,
			[]string{"01-test", "supply", "package-02", "-deploy", "4_test", "2-supply"},
			getEntries(t, path))
	})

	t.Run("los archivos sueltos se ignoran: un step es un directorio", func(t *testing.T) {
		path := pipelineConSteps(t, "01-test")
		require.NoError(t, os.WriteFile(
			filepath.Join(path, "steps", "02-noesundirectorio"), []byte("x"), 0o644))

		assert.Equal(t, []string{"01-test"}, getEntries(t, path))
	})

	t.Run("sin directorio steps devuelve vacío SIN error", func(t *testing.T) {
		ctx := context.Background()
		entries, err := infraPipeline.NewPipelineStepRepository().Get(&ctx, t.TempDir())

		require.NoError(t, err)
		assert.Empty(t, entries)
		assert.NotNil(t, entries, "devuelve slice vacío, no nil")
	})

	t.Run("directorio steps vacío devuelve vacío", func(t *testing.T) {
		path := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(path, "steps"), 0o755))

		assert.Empty(t, getEntries(t, path))
	})
}

func TestPipelineStepRepository_Get_ElRepositorioNoConstruyeStepNames(t *testing.T) {
	// Canario del contrato: el tipo devuelto es el nombre crudo. Si alguien
	// vuelve a meter `command.NewStepName` aquí, la firma cambia y esto no
	// compila.
	ctx := context.Background()
	entries, err := infraPipeline.NewPipelineStepRepository().Get(&ctx, pipelineConSteps(t, "2-supply"))

	require.NoError(t, err)
	require.Equal(t, []domPipeline.StepEntry{"2-supply"}, entries,
		"el nombre llega intacto al validador, sin normalizar a 02-supply")
}
