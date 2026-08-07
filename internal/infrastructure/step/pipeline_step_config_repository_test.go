package step_test

// El lector de `steps/NN-nombre/config.yaml` (spec 13 §5.1 y §5.3).
//
// Lo que este adaptador tiene que distinguir son TRES situaciones que desde
// fuera se parecen: el archivo no está, el archivo está y declara, y el archivo
// está y no declara nada válido. Sólo la tercera es un error, y la diferencia
// entre la primera y la tercera es la mitad de §5.3 que es fácil de perder.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	infraStep "github.com/jairoprogramador/vex-engine/internal/infrastructure/step"
)

// pipelineConConfig materializa un pipelinecode con el `config.yaml` de un step.
// Un valor nil significa que el step existe y el archivo NO.
func pipelineConConfig(t *testing.T, step string, contenido *string) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "steps", step), 0o755))
	if contenido != nil {
		require.NoError(t, os.WriteFile(
			filepath.Join(root, "steps", step, "config.yaml"), []byte(*contenido), 0o644))
	}
	return root
}

func leerConfig(t *testing.T, root, step string) (domStep.StepConfig, error) {
	t.Helper()
	ctx := context.Background()
	return infraStep.NewPipelineStepConfigRepository().Get(&ctx, root, step)
}

func TestPipelineStepConfigRepository_Get(t *testing.T) {
	contenido := func(s string) *string { return &s }

	t.Run("sin config.yaml no hay ámbito, y NO es un error", func(t *testing.T) {
		// Es lo que hace legítimo un step sin declarar: se ejecutará siempre y no
		// persistirá registro. Si esto devolviera error, los tres templates reales
		// dejarían de arrancar antes de la spec 24.
		config, err := leerConfig(t, pipelineConConfig(t, "02-supply", nil), "02-supply")

		require.NoError(t, err)
		assert.False(t, config.IsDeclared())
	})

	t.Run("sin el directorio del step tampoco", func(t *testing.T) {
		config, err := leerConfig(t, t.TempDir(), "02-supply")

		require.NoError(t, err)
		assert.False(t, config.IsDeclared())
	})

	t.Run("los dos ámbitos del vocabulario", func(t *testing.T) {
		for _, declarado := range []string{"project", "environment"} {
			t.Run(declarado, func(t *testing.T) {
				root := pipelineConConfig(t, "01-acr", contenido("scope: "+declarado+"\n"))

				config, err := leerConfig(t, root, "01-acr")

				require.NoError(t, err)
				require.True(t, config.IsDeclared())
				assert.Equal(t, declarado, config.Scope().String())
			})
		}
	})

	t.Run("los comentarios y el espacio no estorban", func(t *testing.T) {
		root := pipelineConConfig(t, "01-acr", contenido(
			"# el registro de contenedores es común a todos los ambientes\nscope: project\n"))

		config, err := leerConfig(t, root, "01-acr")

		require.NoError(t, err)
		assert.True(t, config.Scope().IsProject())
	})

	// El archivo PRESENTE es una intención de declarar: tratarlo como ausente
	// sería adivinar cuál, que es justo lo que esta spec retira.
	t.Run("presente y sin ámbito válido es un error que nombra el archivo", func(t *testing.T) {
		casos := []struct {
			nombre    string
			contenido string
			enElError string
		}{
			{"vacío", "", "no declara 'scope'"},
			{"sólo un comentario", "# todavía no lo decido\n", "no declara 'scope'"},
			{"otro campo", "max_age: 30d\n", "no declara 'scope'"},
			{"ámbito inventado", "scope: shared\n", "shared"},
			{"scope vacío", "scope: \"\"\n", "no declara 'scope'"},
			{"YAML roto", "scope: [project\n", "parsear YAML"},
		}

		for _, caso := range casos {
			t.Run(caso.nombre, func(t *testing.T) {
				root := pipelineConConfig(t, "02-supply", contenido(caso.contenido))

				_, err := leerConfig(t, root, "02-supply")

				require.Error(t, err)
				assert.Contains(t, err.Error(), "steps/02-supply/config.yaml",
					"quien lee esto edita el pipelinecode: el error tiene que nombrar el archivo")
				assert.Contains(t, err.Error(), caso.enElError)
			})
		}
	})

	// La ruta del error es la RELATIVA al pipelinecode, no la absoluta del
	// directorio temporal en el que el motor lo clonó: nadie edita ahí.
	t.Run("el error no filtra la ruta de trabajo del motor", func(t *testing.T) {
		root := pipelineConConfig(t, "02-supply", contenido("scope: shared\n"))

		_, err := leerConfig(t, root, "02-supply")

		require.Error(t, err)
		assert.NotContains(t, err.Error(), root)
	})
}
