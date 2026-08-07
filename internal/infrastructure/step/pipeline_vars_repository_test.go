package step_test

// El lector de `variables/<ambiente>/<paso>.yaml` y la gramática de la spec 14
// §5.2.
//
// Lo que este adaptador dejó de hacer es tan importante como lo que hace: ya no
// devuelve variables ya resueltas. Devuelve DECLARACIONES, porque una variable
// con nombre y valor no puede decir de dónde salió, y ahí se perdía la mitad del
// concepto que esta spec separa.

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

func pipelineConVariables(t *testing.T, environment, step, contenido string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "variables", environment)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, step+".yaml"), []byte(contenido), 0o644))
	return root
}

func leerDeclaraciones(t *testing.T, root, environment, step string) ([]domStep.VariableDeclaration, error) {
	t.Helper()
	ctx := context.Background()
	return infraStep.NewPipelineVarsRepository().Get(&ctx, root, environment, step)
}

func TestPipelineVarsRepository_LaGramaticaDeLasTresFormas(t *testing.T) {
	root := pipelineConVariables(t, "prod", "package", `
# (a) literal — sigue existiendo, y sigue rigiéndose por la spec 12
- name: "instance_count"
  value: "3"

# (b) producido por el output de un step de este pipeline
- name: "acr_login_server"
  resolve: step-output
  from: "02-supply"
  key: "azure_container_registry_login_server"

# (c) leído del registro de estado de otro ámbito
- name: "DB_HOST"
  resolve: state
  scope: project
  key: "lb-arn"
`)

	declaraciones, err := leerDeclaraciones(t, root, "prod", "package")

	require.NoError(t, err)
	require.Len(t, declaraciones, 3)

	assert.True(t, declaraciones[0].IsLiteral())
	assert.Equal(t, "instance_count", declaraciones[0].Name())
	assert.Equal(t, "3", declaraciones[0].Value())

	assert.Equal(t, domStep.SourceStepOutput, declaraciones[1].Source())
	assert.Equal(t, "02-supply", declaraciones[1].From())
	assert.Equal(t, "azure_container_registry_login_server", declaraciones[1].Key())

	assert.Equal(t, domStep.SourceState, declaraciones[2].Source())
	assert.Equal(t, "project", declaraciones[2].Scope().String())
	assert.Equal(t, "lb-arn", declaraciones[2].Key())
}

func TestPipelineVarsRepository_LoQueSeAceptaYLoQueNo(t *testing.T) {
	t.Run("sin archivo no hay declaraciones, y NO es un error", func(t *testing.T) {
		declaraciones, err := leerDeclaraciones(t, t.TempDir(), "sand", "supply")

		require.NoError(t, err)
		assert.Empty(t, declaraciones)
	})

	// Herencia de la spec 03 §5.1: «declarada y vacía» y «no declarada» son
	// estados distintos y producen material de identidad distinto. La asimetría
	// con un `outputs` —donde un grupo de captura vacío SÍ es un fallo— la
	// conserva `CommandVariable`, y la gramática tiene que decirla porque para
	// quien escribe el pipelinecode las dos son «una variable».
	t.Run("un literal con value vacío es legítimo", func(t *testing.T) {
		root := pipelineConVariables(t, "sand", "supply", "- name: instance_count\n  value: \"\"\n")

		declaraciones, err := leerDeclaraciones(t, root, "sand", "supply")

		require.NoError(t, err)
		require.Len(t, declaraciones, 1)
		assert.True(t, declaraciones[0].IsLiteral())
		assert.Equal(t, "", declaraciones[0].Value())
	})

	t.Run("lo que el archivo no puede decir", func(t *testing.T) {
		casos := []struct {
			nombre    string
			contenido string
			enElError string
		}{
			{"sin name", "- value: \"3\"\n", "name"},
			{"resolve inventado", "- name: acr\n  resolve: dynamic\n", "'resolve: dynamic'"},
			{"step-output sin from", "- name: acr\n  resolve: step-output\n  key: k\n", "'from'"},
			{"step-output sin key", "- name: acr\n  resolve: step-output\n  from: 02-supply\n", "'key'"},
			{"state sin scope", "- name: acr\n  resolve: state\n  key: k\n", "'scope'"},
			{"state con ámbito inventado", "- name: acr\n  resolve: state\n  scope: global\n  key: k\n", "global"},
			{"YAML roto", "- name: [acr\n", "parsear YAML"},
		}

		for _, caso := range casos {
			t.Run(caso.nombre, func(t *testing.T) {
				root := pipelineConVariables(t, "sand", "supply", caso.contenido)

				_, err := leerDeclaraciones(t, root, "sand", "supply")

				require.Error(t, err)
				assert.Contains(t, err.Error(), caso.enElError)
				assert.Contains(t, err.Error(), "supply.yaml",
					"el error nombra el archivo que hay que editar")
			})
		}
	})
}
