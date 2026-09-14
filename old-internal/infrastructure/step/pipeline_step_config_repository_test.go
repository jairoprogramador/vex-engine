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

	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
	infraStep "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/step"
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

// ── La gramática de `rules` (spec 15) ───────────────────────────────────────

// Las dos formas conviven en el mismo archivo, y la corta no es azúcar: dice
// «vigílalo todo», que NO es lo mismo que una lista vacía.
func TestPipelineStepConfigRepository_LaGramaticaDeRules(t *testing.T) {
	contenido := func(s string) *string { return &s }

	leerReglas := func(t *testing.T, cuerpo string) domStep.RuleSet {
		t.Helper()
		root := pipelineConConfig(t, "01-acr", contenido(cuerpo))
		config, err := leerConfig(t, root, "01-acr")
		require.NoError(t, err)
		return config.Rules()
	}

	t.Run("sin la clave rules el conjunto está vacío", func(t *testing.T) {
		// Es la mitad de §5.5 que hace que un pipelinecode de la spec 13 siga
		// arrancando: se ejecuta siempre, que es más lento que ayer y nunca
		// incorrecto.
		assert.True(t, leerReglas(t, "scope: project\n").IsEmpty())
	})

	t.Run("rules vacío significa lo mismo", func(t *testing.T) {
		assert.True(t, leerReglas(t, "scope: project\nrules: []\n").IsEmpty())
	})

	t.Run("la forma corta vigila también el proyecto", func(t *testing.T) {
		regla, declarada := leerReglas(t,
			"scope: project\nrules:\n  - state_changed\n").StateChanged()

		require.True(t, declarada)
		assert.True(t, regla.WatchesProject())
	})

	t.Run("la forma larga con una sola fuente lo deja fuera", func(t *testing.T) {
		regla, declarada := leerReglas(t,
			"scope: project\nrules:\n  - state_changed: [pipeline]\n").StateChanged()

		require.True(t, declarada)
		assert.False(t, regla.WatchesProject())
	})

	t.Run("las dos reglas juntas, en su orden", func(t *testing.T) {
		rules := leerReglas(t,
			"scope: environment\nrules:\n  - state_changed: [pipeline, project]\n  - max_age: 24h\n")

		declaradas := rules.Rules()
		require.Len(t, declaradas, 2)
		assert.Equal(t, domStep.RuleKindStateChanged, declaradas[0].Kind())
		assert.Equal(t, domStep.RuleKindMaxAge, declaradas[1].Kind())
	})

	t.Run("la duración entre comillas o sin ellas", func(t *testing.T) {
		for _, cuerpo := range []string{
			"scope: environment\nrules:\n  - max_age: 6h\n",
			"scope: environment\nrules:\n  - max_age: \"6h\"\n",
			"scope: environment\nrules:\n  - max_age: '6h'\n",
		} {
			rules := leerReglas(t, cuerpo)
			require.Len(t, rules.Rules(), 1)
			assert.True(t, rules.ExpiresWithoutInvalidating())
		}
	})

	t.Run("los comentarios no estorban", func(t *testing.T) {
		rules := leerReglas(t, `
scope: project
rules:
  # el ACR no depende del código de la aplicación
  - state_changed: [pipeline]
`)
		require.Len(t, rules.Rules(), 1)
	})
}

// El vocabulario es CERRADO por los dos lados —las reglas y sus fuentes— y todo
// lo que no encaja es un error de pipelinecode, no una regla que se ignora.
//
// Ignorarla sería peor: quien la escribió creería que su step tiene una
// comprobación que no tiene, y el motor no puede decirle que no.
func TestPipelineStepConfigRepository_UnaReglaMalDeclaradaEsUnError(t *testing.T) {
	contenido := func(s string) *string { return &s }

	casos := []struct {
		nombre    string
		cuerpo    string
		enElError string
	}{
		{
			nombre:    "una regla desconocida",
			cuerpo:    "scope: project\nrules:\n  - content_changed\n",
			enElError: "content_changed",
		},
		{
			nombre:    "state_changed sin pipeline",
			cuerpo:    "scope: project\nrules:\n  - state_changed: [project]\n",
			enElError: "no puede omitir 'pipeline'",
		},
		{
			nombre:    "una fuente inventada",
			cuerpo:    "scope: project\nrules:\n  - state_changed: [pipeline, templates]\n",
			enElError: "templates",
		},
		{
			nombre:    "max_age sin duración",
			cuerpo:    "scope: project\nrules:\n  - max_age\n",
			enElError: "necesita una duración",
		},
		{
			nombre:    "max_age en días",
			cuerpo:    "scope: project\nrules:\n  - max_age: 30d\n",
			enElError: "720h",
		},
		{
			nombre:    "state_changed con una duración",
			cuerpo:    "scope: project\nrules:\n  - state_changed: 24h\n",
			enElError: "lista de fuentes",
		},
		{
			nombre:    "dos reglas en un mismo elemento",
			cuerpo:    "scope: project\nrules:\n  - {state_changed: [pipeline], max_age: 24h}\n",
			enElError: "declara UNA regla",
		},
		{
			nombre:    "la misma regla dos veces",
			cuerpo:    "scope: project\nrules:\n  - max_age: 1h\n  - max_age: 24h\n",
			enElError: "dos veces",
		},
		{
			nombre:    "rules no es una lista",
			cuerpo:    "scope: project\nrules: state_changed\n",
			enElError: "parsear YAML",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			root := pipelineConConfig(t, "02-supply", contenido(caso.cuerpo))

			_, err := leerConfig(t, root, "02-supply")

			require.Error(t, err)
			assert.Contains(t, err.Error(), "steps/02-supply/config.yaml",
				"quien lee esto edita el pipelinecode: el error tiene que nombrar el archivo")
			assert.Contains(t, err.Error(), caso.enElError)
		})
	}
}
