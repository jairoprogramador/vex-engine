package pipeline_test

// El lector de `vexpipeline.yaml` (spec 14 §5.1, cierra D-A16).
//
// Distingue las mismas tres situaciones que el lector del `config.yaml` de un
// step, y por la misma razón: la AUSENCIA es un estado legítimo —la versión 1, o
// sea todo el pipelinecode escrito hasta hoy— y un archivo PRESENTE es una
// intención de declarar, así que uno que no declara una versión conocida no se
// puede tratar como ausente sin adivinar cuál.

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	infraPipeline "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
)

func pipelineConManifiesto(t *testing.T, contenido *string) string {
	t.Helper()
	root := t.TempDir()
	if contenido != nil {
		require.NoError(t, os.WriteFile(
			filepath.Join(root, "vexpipeline.yaml"), []byte(*contenido), 0o644))
	}
	return root
}

func leerManifiesto(t *testing.T, root string) (domPipeline.Manifest, error) {
	t.Helper()
	ctx := context.Background()
	return infraPipeline.NewPipelineManifestRepository().Get(&ctx, root)
}

func TestPipelineManifestRepository_Get(t *testing.T) {
	contenido := func(s string) *string { return &s }

	t.Run("sin manifiesto es la versión 1, y NO es un error", func(t *testing.T) {
		manifest, err := leerManifiesto(t, pipelineConManifiesto(t, nil))

		require.NoError(t, err)
		assert.False(t, manifest.IsDeclared())
		assert.Equal(t, domPipeline.SchemaVersion1, manifest.SchemaVersion())
		assert.False(t, manifest.AllowsDeclaredSources(),
			"en la versión 1 `resolve` no existe")
	})

	t.Run("las dos versiones del vocabulario", func(t *testing.T) {
		for _, caso := range []struct {
			version int
			permite bool
		}{
			{domPipeline.SchemaVersion1, false},
			{domPipeline.SchemaVersion2, true},
		} {
			t.Run(strconv.Itoa(caso.version), func(t *testing.T) {
				root := pipelineConManifiesto(t, contenido(
					"# el contrato del formato\nschema_version: "+strconv.Itoa(caso.version)+"\n"))

				manifest, err := leerManifiesto(t, root)

				require.NoError(t, err)
				assert.True(t, manifest.IsDeclared())
				assert.Equal(t, caso.version, manifest.SchemaVersion())
				assert.Equal(t, caso.permite, manifest.AllowsDeclaredSources())
			})
		}
	})

	t.Run("presente y sin versión conocida es un error que nombra el archivo", func(t *testing.T) {
		casos := []struct {
			nombre    string
			contenido string
			enElError string
		}{
			{"vacío", "", "no declara 'schema_version'"},
			{"sólo un comentario", "# todavía no\n", "no declara 'schema_version'"},
			{"otro campo", "name: mi-pipeline\n", "no declara 'schema_version'"},
			{"una versión del futuro", "schema_version: 99\n", "schema_version: 99"},
			{"YAML roto", "schema_version: [2\n", "parsear YAML"},
		}

		for _, caso := range casos {
			t.Run(caso.nombre, func(t *testing.T) {
				_, err := leerManifiesto(t, pipelineConManifiesto(t, contenido(caso.contenido)))

				require.Error(t, err)
				assert.Contains(t, err.Error(), "vexpipeline.yaml",
					"quien lee esto edita el pipelinecode: el error tiene que nombrar el archivo")
				assert.Contains(t, err.Error(), caso.enElError)
			})
		}
	})
}

// LA VENTANA DE REUTILIZACIÓN DEL CLON (spec 18 §5.4).
//
// Se declara en el pipelinecode y no en el motor porque quien sabe con qué
// frecuencia cambia un pipeline es quien lo escribe. Lo que el motor pone es el
// DEFAULT, y lo pone en un solo sitio.
func TestPipelineManifestRepository_VentanaDeClon(t *testing.T) {
	contenido := func(s string) *string { return &s }

	t.Run("sin declarar es la de por defecto", func(t *testing.T) {
		for _, caso := range []struct {
			nombre     string
			manifiesto *string
		}{
			{"sin manifiesto", nil},
			{"con manifiesto y sin la clave", contenido("schema_version: 1\n")},
			{"con la clave vacía", contenido("schema_version: 1\nclone_window: \"\"\n")},
		} {
			t.Run(caso.nombre, func(t *testing.T) {
				manifest, err := leerManifiesto(t, pipelineConManifiesto(t, caso.manifiesto))

				require.NoError(t, err)
				assert.Equal(t, domPipeline.DefaultCloneWindow, manifest.CloneWindow())
				assert.False(t, manifest.DeclaresCloneWindow())
			})
		}
	})

	t.Run("declarada gana", func(t *testing.T) {
		manifest, err := leerManifiesto(t,
			pipelineConManifiesto(t, contenido("schema_version: 1\nclone_window: 15m\n")))

		require.NoError(t, err)
		assert.Equal(t, 15*time.Minute, manifest.CloneWindow())
		assert.True(t, manifest.DeclaresCloneWindow())
	})

	t.Run("presente e ilegible es un error, no el default", func(t *testing.T) {
		for _, declarada := range []string{"un rato", "24", "-24h"} {
			_, err := leerManifiesto(t,
				pipelineConManifiesto(t, contenido("schema_version: 1\nclone_window: "+declarada+"\n")))

			require.Error(t, err, "se esperaba error para %q", declarada)
			assert.Contains(t, err.Error(), "vexpipeline.yaml")
		}
	})
}
