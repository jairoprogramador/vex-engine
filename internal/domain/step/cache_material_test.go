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

// materialDe compone el material tal como lo hace el handler 03, sobre un
// contexto con las variables y el status que se le den, para un step que vigila
// TODO: `- state_changed` a secas.
func materialDe(
	t *testing.T,
	commands []command.Command,
	preparar func(*command.ExecutionContext),
) domCache.Material {
	t.Helper()
	return materialVigilando(t, domStep.NewDefaultStateChangedRule(), commands, preparar)
}

// materialVigilando es lo mismo con la regla `state_changed` que se le diga: es
// lo que decide si el código del proyecto entra en el material (spec 15 §5.2).
func materialVigilando(
	t *testing.T,
	watched domStep.StateChangedRule,
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
	material, err := domStep.NewCacheMaterial(request, commands, watched)
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
	v, err := command.NewVariable(name, value, command.OriginDeclared)
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

// EL TÉRMINO CONDICIONAL (spec 15 §5.2): un step que declara
// `state_changed: [pipeline]` afirma que su trabajo no depende del código de la
// aplicación, y su huella lo refleja.
//
// Es la mitad que hace observable la granularidad declarada. La otra —que la
// regla evalúa igual en los dos casos— está en `rule_test.go`: la fuente no
// cambia lo que se compara, cambia lo que se compone.
func TestNewCacheMaterial_ElCodigoDelProyectoEsElTerminoCondicional(t *testing.T) {
	soloPipeline, err := domStep.NewStateChangedRule([]string{"pipeline"})
	require.NoError(t, err)

	material := materialVigilando(
		t, soloPipeline, []command.Command{comandoDePrueba(t)}, nil)

	assert.True(t, material.CodeExcluded)
	assert.True(t, material.Code.IsZero())
	require.NoError(t, material.Validate(),
		"el hueco está DECLARADO, que es lo que lo distingue de un material incompleto")

	conProyecto := materialDe(t, []command.Command{comandoDePrueba(t)}, nil)
	assert.NotEqual(t, conProyecto, material,
		"dos alcances distintos no pueden producir la misma identidad")

	// Y la forma corta es exactamente la forma larga completa: es la comprobación
	// del default de §5.2, y el sitio donde un cambio de criterio se vería.
	completa, err := domStep.NewStateChangedRule([]string{"pipeline", "project"})
	require.NoError(t, err)
	assert.Equal(t, conProyecto,
		materialVigilando(t, completa, []command.Command{comandoDePrueba(t)}, nil))
}

// El material del árbol se PARSEA, no se copia: un `ProjectStatus` corrupto o
// vacío falla aquí —y el paso se ejecuta— en vez de colarse en la clave.
//
// Salvo que el step no lo vigile, y entonces ni se mira: un `01-acr` con
// `state_changed: [pipeline]` no puede fallar por una huella de código que no
// forma parte de su identidad.
func TestNewCacheMaterial_UnaHuellaDeArbolInvalidaEsUnError(t *testing.T) {
	for _, invalida := range []string{"", "sin-prefijo", "v1:corta"} {
		t.Run(invalida, func(t *testing.T) {
			contexto := contextoDePrueba(t)
			contexto.SetProjectStatus(invalida)
			request := domStep.NewStepRequestHandler(contexto, contexto.StepName())

			_, err := domStep.NewCacheMaterial(
				request, []command.Command{comandoDePrueba(t)},
				domStep.NewDefaultStateChangedRule())

			assert.Error(t, err)

			soloPipeline, err := domStep.NewStateChangedRule([]string{"pipeline"})
			require.NoError(t, err)
			_, err = domStep.NewCacheMaterial(
				request, []command.Command{comandoDePrueba(t)}, soloPipeline)

			assert.NoError(t, err,
				"quien no vigila el código del proyecto tampoco depende de que se pueda leer")
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

// El ORIGEN de una variable no entra en la huella (spec 12 §5.5).
//
// Lo que identifica es el par (nombre, valor) resultante, no por dónde llegó:
// dos ejecuciones que alcanzan el mismo valor por caminos distintos son la misma
// configuración. Es la misma regla que P9 aplica al código —la huella
// identifica, el commit documenta— y está escrita en
// `fingerprint/SPEC-VARIABLES-v1.md` §4, que es donde la buscará quien lea la
// especificación de la huella.
func TestNewCacheMaterial_ElOrigenNoEntraEnLaHuellaDeVariables(t *testing.T) {
	huellaCon := func(origen command.Origin) string {
		material := materialDe(t, nil, func(contexto *command.ExecutionContext) {
			v, err := command.NewVariable("acr_name", "acme.azurecr.io", origen)
			require.NoError(t, err)
			contexto.AddAccumulatedVar(v)
		})
		return material.Variables.String()
	}

	declarada := huellaCon(command.OriginDeclared)
	for _, origen := range []command.Origin{
		command.OriginState, command.OriginInjected,
		command.OriginResolved, command.OriginRuntime,
	} {
		assert.Equal(t, declarada, huellaCon(origen),
			"el origen %s cambió la huella", origen)
	}
}

// ── La declaración entra, el valor resuelto no (spec 14 §5.3) ───────────────

// huellaDeDeclaracion compone el material de un step que declara UNA variable con
// fuente, ya resuelta al valor que se le dé.
func huellaDeDeclaracion(
	t *testing.T,
	declaracion domStep.VariableDeclaration,
	valorResuelto string,
) string {
	t.Helper()

	contexto := contextoDePrueba(t)
	contexto.SetProjectStatus(huellaDeArbol)

	resuelta, err := command.NewVariable(declaracion.Name(), valorResuelto, command.OriginResolved)
	require.NoError(t, err)
	contexto.AddAccumulatedVar(resuelta)

	request := domStep.NewStepRequestHandler(contexto, contexto.StepName())
	request.SetSourcedDeclarations([]domStep.VariableDeclaration{declaracion})

	material, err := domStep.NewCacheMaterial(
		request, nil, domStep.NewDefaultStateChangedRule())
	require.NoError(t, err)
	return material.Variables.String()
}

// LA regla que gobierna el resto del catálogo: en la identidad entra la
// declaración, nunca el valor.
//
// Es la precondición dura de todo el registro. Un valor producido en runtime no
// se puede saber por adelantado; la declaración de cómo se obtiene sí, y por eso
// un identificador se puede componer ANTES de ejecutar (spec 18). Sin esto, un
// `content_id` no podría distinguir «parámetro con valor conocido» de «parámetro
// que se resolverá», y quedaría incompleto EN SILENCIO —la peor manera de quedar
// incompleto—.
func TestNewCacheMaterial_DeUnaDeclaracionEntraLaDeclaracionYNoElValor(t *testing.T) {
	declaracion, err := domStep.NewStepOutputDeclaration("acr", "02-supply", "acr_name")
	require.NoError(t, err)

	t.Run("dos ejecuciones que resuelven valores distintos dan el mismo material", func(t *testing.T) {
		assert.Equal(t,
			huellaDeDeclaracion(t, declaracion, "acme.azurecr.io"),
			huellaDeDeclaracion(t, declaracion, "otra.azurecr.io"),
			"el valor resuelto se registra aparte, como hecho (spec 19); aquí no entra")
	})

	// Discriminación: es el argumento contra el marcador genérico `dynamic: true`
	// de la alternativa B. Un «esto se resuelve luego» produciría la MISMA cadena
	// para los tres casos de abajo, y una parte constante de un hash no aporta
	// identidad.
	t.Run("cambiar la declaración cambia el material", func(t *testing.T) {
		base := huellaDeDeclaracion(t, declaracion, "acme.azurecr.io")

		otroFrom, err := domStep.NewStepOutputDeclaration("acr", "01-test", "acr_name")
		require.NoError(t, err)
		otroKey, err := domStep.NewStepOutputDeclaration("acr", "02-supply", "acr_login_server")
		require.NoError(t, err)
		otraFuente, err := domStep.NewStateDeclaration("acr", "project", "acr_name")
		require.NoError(t, err)

		for nombre, distinta := range map[string]domStep.VariableDeclaration{
			"otro from":   otroFrom,
			"otro key":    otroKey,
			"otra fuente": otraFuente,
		} {
			t.Run(nombre, func(t *testing.T) {
				assert.NotEqual(t, base, huellaDeDeclaracion(t, distinta, "acme.azurecr.io"))
			})
		}
	})

	// Y un literal sigue entrando por su VALOR, que es lo que siempre hizo: su
	// declaración y su valor son la misma cosa, así que no hay nada que sustituir
	// y ninguna huella ya emitida se mueve por esto.
	t.Run("un literal entra por su valor", func(t *testing.T) {
		conValor := materialDe(t, nil, func(contexto *command.ExecutionContext) {
			contexto.AddAccumulatedVar(variable(t, "acr", "acme.azurecr.io"))
		})
		otroValor := materialDe(t, nil, func(contexto *command.ExecutionContext) {
			contexto.AddAccumulatedVar(variable(t, "acr", "otra.azurecr.io"))
		})

		assert.NotEqual(t, conValor.Variables.String(), otroValor.Variables.String())
	})
}
