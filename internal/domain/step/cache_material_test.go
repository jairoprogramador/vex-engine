package step_test

// La traducción del modelo de ejecución al material de la clave (spec 10 §5.1).
//
// Lo que se prueba aquí no son las tres reglas de huella —eso está en
// `fingerprint`, con sus vectores— sino que ESTE archivo les entregue lo que
// deben ver: el ámbito en su sitio, el filtro de volátiles aplicado, `show`
// dentro y la huella del árbol con su prefijo.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domCache "github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

const huellaDeArbol = "v1:1111111111111111111111111111111111111111111111111111111111111111"

// materialDe compone el material tal como lo hace el handler 04, sobre un
// contexto con las variables y el status que se le den.
func materialDe(
	t *testing.T,
	commands []command.Command,
	preparar func(*command.ExecutionContext),
) domCache.Material {
	t.Helper()

	contexto := contextoDePrueba(t)
	contexto.SetProjectStatus(huellaDeArbol)
	if preparar != nil {
		preparar(contexto)
	}

	request := domStep.NewStepRequestHandler(contexto, contexto.StepName())
	material, err := domStep.NewCacheMaterial(request, commands)
	require.NoError(t, err)
	return material
}

func comandoDePrueba(t *testing.T, opts ...command.CommandOption) command.Command {
	t.Helper()
	cmd, err := command.NewCommand("provision", "echo hola", opts...)
	require.NoError(t, err)
	return cmd
}

func variable(t *testing.T, name, value string) command.Variable {
	t.Helper()
	v, err := command.NewVariable(name, value, false)
	require.NoError(t, err)
	return v
}

// Las cuatro dimensiones no-huella salen de donde deben.
func TestNewCacheMaterial_LasDimensionesSalenDeLaEjecucion(t *testing.T) {
	material := materialDe(t, []command.Command{comandoDePrueba(t)}, nil)

	assert.Equal(t, "https://vex.test/org/proyecto.git", material.Subject)
	assert.Equal(t, "https://vex.test/org/pipeline.git", material.Pipeline)
	assert.Equal(t, "prod", material.Scope,
		"hasta la spec 15 el ámbito ES el ambiente, y va en la clave por derecho propio")
	assert.Equal(t, "supply", material.Step)
	assert.Equal(t, huellaDeArbol, material.Code.String(),
		"la huella del árbol viaja con su prefijo de versión, no pelada")

	require.NoError(t, material.Validate())
}

// El material del árbol se PARSEA, no se copia: un `ProjectStatus` corrupto o
// vacío falla aquí —y el paso se ejecuta— en vez de colarse en la clave.
func TestNewCacheMaterial_UnaHuellaDeArbolInvalidaEsUnError(t *testing.T) {
	for _, invalida := range []string{"", "sin-prefijo", "v1:corta"} {
		t.Run(invalida, func(t *testing.T) {
			contexto := contextoDePrueba(t)
			contexto.SetProjectStatus(invalida)
			request := domStep.NewStepRequestHandler(contexto, contexto.StepName())

			_, err := domStep.NewCacheMaterial(request, []command.Command{comandoDePrueba(t)})

			assert.Error(t, err)
		})
	}
}

// El filtro de volátiles se aplica AQUÍ, con la lista que declara
// `command.VolatileVarNames` y que `fingerprint/SPEC-VARIABLES-v1.md` §3.1
// transcribe. Sin él ningún paso se saltaría jamás: las seis cambian solas.
func TestNewCacheMaterial_LasVariablesVolatilesNoEntran(t *testing.T) {
	base := materialDe(t, []command.Command{comandoDePrueba(t)}, nil)

	for _, volatil := range command.VolatileVarNames() {
		t.Run(volatil, func(t *testing.T) {
			conVolatil := materialDe(t, []command.Command{comandoDePrueba(t)},
				func(c *command.ExecutionContext) {
					c.AddAccumulatedVar(variable(t, volatil, "cambia-en-cada-corrida"))
				})

			assert.Equal(t, base.Variables.String(), conVolatil.Variables.String())
		})
	}

	t.Run("una variable cualquiera SÍ entra", func(t *testing.T) {
		conVariable := materialDe(t, []command.Command{comandoDePrueba(t)},
			func(c *command.ExecutionContext) {
				c.AddAccumulatedVar(variable(t, "region", "us-east-1"))
			})

		assert.NotEqual(t, base.Variables.String(), conVariable.Variables.String())
	})

	t.Run("`environment` SÍ entra, pero no es lo que aísla los ambientes", func(t *testing.T) {
		conAmbiente := materialDe(t, []command.Command{comandoDePrueba(t)},
			func(c *command.ExecutionContext) {
				c.AddAccumulatedVar(variable(t, command.VarEnvironment, "prod"))
			})

		assert.NotEqual(t, base.Variables.String(), conAmbiente.Variables.String())
		// El aislamiento lo da `Scope`, que está en la clave aunque esta huella
		// no llevara el ambiente: ver TestNewCacheKey_ElAmbienteNoSePuedeCaerDeLaClave.
		assert.Equal(t, "prod", conAmbiente.Scope)
	})
}

// `show` llega a la huella de instrucciones (spec 10 §5.1bis). Es el otro
// extremo del test de unidad de la regla: aquí se comprueba que el traductor no
// lo pierda por el camino, que es exactamente lo que pasaba antes.
func TestNewCacheMaterial_ShowLlegaALaHuellaDeInstrucciones(t *testing.T) {
	sinShow := materialDe(t, []command.Command{comandoDePrueba(t)}, nil)
	conShow := materialDe(t,
		[]command.Command{comandoDePrueba(t, command.WithShow(true))}, nil)

	assert.NotEqual(t, sinShow.Instructions.String(), conShow.Instructions.String())
}

// El material completo produce una clave, y dos materiales iguales la misma.
func TestNewCacheMaterial_ProduceUnaClaveEstable(t *testing.T) {
	primera, err := domCache.NewCacheKey(materialDe(t, []command.Command{comandoDePrueba(t)}, nil))
	require.NoError(t, err)
	segunda, err := domCache.NewCacheKey(materialDe(t, []command.Command{comandoDePrueba(t)}, nil))
	require.NoError(t, err)

	assert.True(t, primera.Equals(segunda))
}
