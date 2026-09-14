package step_test

// La traducción del modelo de ejecución al material de las dos reglas de huella
// (spec 27 §5.2').
//
// Lo que se prueba aquí no son las reglas —eso está en `fingerprint`, con sus
// vectores— sino que ESTE archivo les entregue lo que deben ver: el `config.yaml`
// entero, `show` dentro, las DECLARACIONES en lugar del mapa acumulado resuelto,
// y la huella del árbol del proyecto con su prefijo y sólo cuando el step la
// vigila.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
	infraFingerprint "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/fingerprint"
)

const huellaDeArbol = "v1:1111111111111111111111111111111111111111111111111111111111111111"

// arbolDelStep es el directorio de un step vacío de todo menos sus archivos de
// declaración, que la regla excluye.
func arbolDelStep() domFingerprint.TreeSource {
	return infraFingerprint.NewMemTreeSource().
		AddFile("commands.yaml", "- name: provision\n  cmd: echo hola\n")
}

// declaracionDe compone `pipe-v1` tal como lo hace el resolutor del pipeline.
func declaracionDe(
	t *testing.T,
	loaded domStep.LoadedStep,
	tree domFingerprint.TreeSource) domFingerprint.Fingerprint {

	t.Helper()
	if tree == nil {
		tree = arbolDelStep()
	}
	huella, err := domStep.NewDeclarationFingerprint(loaded, tree)
	require.NoError(t, err)
	return huella
}

// huellaDe compone `sf-v1` tal como lo hace el handler 03, para un step que
// vigila TODO: `- state_changed` a secas.
func huellaDe(t *testing.T, loaded domStep.LoadedStep) string {
	t.Helper()
	return huellaVigilando(t, domStep.NewDefaultStateChangedRule(), loaded)
}

// huellaVigilando es lo mismo con la regla `state_changed` que se le diga: es lo
// que decide si el código del proyecto entra (spec 15 §5.2, spec 27 §5.4).
func huellaVigilando(
	t *testing.T, watched domStep.StateChangedRule, loaded domStep.LoadedStep) string {

	t.Helper()
	huella, err := domStep.NewStepFingerprint(
		declaracionDe(t, loaded, nil), huellaDeArbol, watched)
	require.NoError(t, err)
	return huella.String()
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

func literal(t *testing.T, name, value string) domStep.VariableDeclaration {
	t.Helper()
	d, err := domStep.NewLiteralDeclaration(name, value)
	require.NoError(t, err)
	return d
}

func configDePrueba(t *testing.T, scope string, rules ...domStep.Rule) domStep.StepConfig {
	t.Helper()
	ambito, err := domStep.NewScope(scope)
	require.NoError(t, err)
	conjunto, err := domStep.NewRuleSet(rules...)
	require.NoError(t, err)
	config, err := domStep.NewStepConfig(ambito, conjunto)
	require.NoError(t, err)
	return config
}

func stepCargado(t *testing.T) domStep.LoadedStep {
	t.Helper()
	return domStep.LoadedStep{
		Commands: []command.Command{comandoDePrueba(t)},
		Config:   configDePrueba(t, "environment", domStep.NewDefaultStateChangedRule()),
	}
}

// ── El término condicional ──────────────────────────────────────────────────

// EL TÉRMINO CONDICIONAL (spec 15 §5.2, spec 27 §5.4): un step que declara
// `state_changed: [pipeline]` afirma que su trabajo no depende del código de la
// aplicación, y su huella lo refleja.
//
// Es la mitad que hace observable la granularidad declarada. La otra —que la
// regla evalúa igual en los dos casos— está en `rule_test.go`: la fuente no
// cambia lo que se compara, cambia lo que se compone.
func TestNewStepFingerprint_ElCodigoDelProyectoEsElTerminoCondicional(t *testing.T) {
	soloPipeline, err := domStep.NewStateChangedRule([]string{"pipeline"})
	require.NoError(t, err)

	loaded := stepCargado(t)
	sinProyecto := huellaVigilando(t, soloPipeline, loaded)
	conProyecto := huellaDe(t, loaded)

	assert.NotEqual(t, conProyecto, sinProyecto,
		"dos alcances distintos no pueden producir la misma identidad")

	// Y la forma corta es exactamente la forma larga completa: es la comprobación
	// del default de §5.2, y el sitio donde un cambio de criterio se vería.
	completa, err := domStep.NewStateChangedRule([]string{"pipeline", "project"})
	require.NoError(t, err)
	assert.Equal(t, conProyecto, huellaVigilando(t, completa, loaded))
}

// La huella del árbol del proyecto se PARSEA, no se copia: un `ProjectStatus`
// corrupto o vacío falla aquí —y el step se ejecuta— en vez de colarse en la
// huella.
//
// Salvo que el step no lo vigile, y entonces ni se mira: un `02-acr` con
// `state_changed: [pipeline]` no puede fallar por una huella de código que no
// forma parte de su identidad.
func TestNewStepFingerprint_UnaHuellaDeArbolInvalidaEsUnError(t *testing.T) {
	declaracion := declaracionDe(t, stepCargado(t), nil)

	for _, invalida := range []string{"", "sin-prefijo", "v1:corta"} {
		t.Run(invalida, func(t *testing.T) {
			_, err := domStep.NewStepFingerprint(
				declaracion, invalida, domStep.NewDefaultStateChangedRule())
			assert.Error(t, err)

			soloPipeline, err := domStep.NewStateChangedRule([]string{"pipeline"})
			require.NoError(t, err)
			_, err = domStep.NewStepFingerprint(declaracion, invalida, soloPipeline)

			assert.NoError(t, err,
				"quien no vigila el código del proyecto tampoco depende de que se pueda leer")
		})
	}
}

// ── El material de la declaración ───────────────────────────────────────────

// `show` llega a la huella (spec 10 §5.1bis). Es el otro extremo del test de
// unidad de la regla: aquí se comprueba que el traductor no lo pierda por el
// camino, que es exactamente lo que pasaba antes.
func TestNewDeclarationFingerprint_ShowLlega(t *testing.T) {
	sinShow := stepCargado(t)
	conShow := stepCargado(t)
	conShow.Commands = []command.Command{comandoDePrueba(t, command.WithShow(true))}

	assert.NotEqual(t, huellaDe(t, sinShow), huellaDe(t, conShow))
}

// `config.yaml` entra ENTERO (spec 27 §5.2, defecto (d)): si no entrara, cambiar
// las reglas de re-ejecución de un step no movería su huella y el step se
// saltaría CON LAS REGLAS VIEJAS.
func TestNewDeclarationFingerprint_ConfigYamlLlegaEntero(t *testing.T) {
	base := stepCargado(t)

	t.Run("el ámbito declarado cambia la huella", func(t *testing.T) {
		otro := base
		otro.Config = configDePrueba(t, "project", domStep.NewDefaultStateChangedRule())
		assert.NotEqual(t, huellaDe(t, base), huellaDe(t, otro))
	})

	t.Run("las reglas cambian la huella", func(t *testing.T) {
		maxAge, err := domStep.NewMaxAgeRule("720h")
		require.NoError(t, err)

		otro := base
		otro.Config = configDePrueba(t, "environment",
			domStep.NewDefaultStateChangedRule(), maxAge)

		assert.NotEqual(t, huellaDe(t, base), huellaDe(t, otro))
	})

	t.Run("un step sin config.yaml es material legítimo, no incompleto", func(t *testing.T) {
		sin := base
		sin.Config = domStep.NoStepConfig()

		assert.NotEqual(t, huellaDe(t, base), huellaDe(t, sin))
	})

	// La forma corta y la forma larga completa son el MISMO valor de dominio, así
	// que producen la misma cadena: reescribir `- state_changed` como
	// `- state_changed: [pipeline, project]` no re-ejecuta nada.
	t.Run("el azúcar sintáctico de las reglas no mueve la huella", func(t *testing.T) {
		larga, err := domStep.NewStateChangedRule([]string{"project", "pipeline"})
		require.NoError(t, err)

		otro := base
		otro.Config = configDePrueba(t, "environment", larga)

		assert.Equal(t, huellaDe(t, base), huellaDe(t, otro))
	})
}

// El árbol del directorio del step entra: D14, el caso que da nombre a la
// spec 27. Aquí se comprueba que el TRADUCTOR lo entregue —la regla ya lo prueba
// con sus vectores—.
func TestNewDeclarationFingerprint_ElArbolDelDirectorioLlega(t *testing.T) {
	loaded := stepCargado(t)

	conPlantilla, err := domStep.NewDeclarationFingerprint(loaded,
		infraFingerprint.NewMemTreeSource().AddFile("k8s/deployment.yaml", "replicas: 3\n"))
	require.NoError(t, err)

	otraPlantilla, err := domStep.NewDeclarationFingerprint(loaded,
		infraFingerprint.NewMemTreeSource().AddFile("k8s/deployment.yaml", "replicas: 5\n"))
	require.NoError(t, err)

	assert.NotEqual(t, conPlantilla.String(), otraPlantilla.String())
}

// ── El valor no identifica; la declaración sí ───────────────────────────────

// LA regla que gobierna el resto del catálogo: en la identidad entra la
// declaración, nunca el valor.
//
// Es la precondición dura de todo el registro. Un valor producido en runtime no
// se puede saber por adelantado; la declaración de cómo se obtiene sí, y por eso
// un identificador se puede componer ANTES de ejecutar (spec 18).
func TestNewDeclarationFingerprint_EntraLaDeclaracionYNoElValor(t *testing.T) {
	conDeclaraciones := func(declaraciones ...domStep.VariableDeclaration) string {
		loaded := stepCargado(t)
		loaded.Declarations = declaraciones
		return huellaDe(t, loaded)
	}

	declaracion, err := domStep.NewStepOutputDeclaration("acr", "02-supply", "acr_name")
	require.NoError(t, err)

	// Discriminación: es el argumento contra el marcador genérico `dynamic: true`.
	// Un «esto se resuelve luego» produciría la MISMA cadena para los tres casos, y
	// una parte constante de un hash no aporta identidad.
	t.Run("cambiar la declaración cambia el material", func(t *testing.T) {
		base := conDeclaraciones(declaracion)

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
				assert.NotEqual(t, base, conDeclaraciones(distinta))
			})
		}
	})

	// Y editar un literal SÍ mueve la huella, que es la mitad del defecto (c') que
	// esta spec cierra: hasta aquí el literal entraba por su valor RESUELTO, y el
	// almacén lo pisaba con el viejo.
	t.Run("editar un literal declarado mueve la huella", func(t *testing.T) {
		assert.NotEqual(t,
			conDeclaraciones(literal(t, "acr", "acme.azurecr.io")),
			conDeclaraciones(literal(t, "acr", "otra.azurecr.io")))
	})

	t.Run("declarada y vacía no es no declarada", func(t *testing.T) {
		assert.NotEqual(t, conDeclaraciones(), conDeclaraciones(literal(t, "acr", "")))
	})

	t.Run("el orden de entrega no cuenta", func(t *testing.T) {
		a := literal(t, "alpha", "1")
		b := literal(t, "beta", "2")
		assert.Equal(t, conDeclaraciones(a, b), conDeclaraciones(b, a))
	})
}

// EL MAPA ACUMULADO YA NO ES MATERIAL, y ésa es la mitad que cierra los dos
// defectos de signo contrario de la spec 27:
//
//   - las salidas de una corrida dejan de ser entradas de la siguiente, así que
//     un step con `outputs` deja de re-ejecutarse una vez de más (defecto (c));
//   - y un valor producido en runtime deja de entrar en absoluto, con lo que
//     `step_fingerprint` deja de transportar un digest sin sal de los valores de
//     configuración del paso fuera de la organización (spec 20 §8).
//
// Lo afirma la FIRMA: `NewDeclarationFingerprint` toma el material CARGADO y un
// árbol, y no tiene por dónde recibir un `ExecutionVariableMap`. Lo que se
// comprueba aquí es la consecuencia observable de eso —el material no contiene
// ningún valor de runtime— y el observable de extremo a extremo es
// `TestRunCommand_ReejecucionSinCambios`, cuya segunda corrida ya no ejecuta nada.
func TestNewDeclarationFingerprint_NingunValorDeRuntimeEntra(t *testing.T) {
	produce, err := domStep.NewStepOutputDeclaration("acr_name", "02-acr", "acr_name")
	require.NoError(t, err)

	loaded := stepCargado(t)
	loaded.Declarations = []domStep.VariableDeclaration{produce}

	// Y `environment` tampoco tiene por dónde entrar: es una variable que el
	// motor INYECTA y que nadie declara, así que el cambio de material la deja
	// fuera sola. Era la segunda de las dos vías por las que el ambiente entraba en
	// la huella de un step —la primera era la dimensión `Scope` de
	// `cache.Material`— y cerrarla es lo que permite que un step con
	// `scope: project` reviva entre ambientes.
	huella := huellaDe(t, loaded)
	for _, prohibido := range append(command.VolatileVarNames(), command.VarEnvironment) {
		assert.NotContains(t, huella, prohibido)
	}
	assert.Equal(t, huella, huellaDe(t, loaded))
}

// ── El material de identidad se calcula UNA vez ─────────────────────────────

// La misma declaración produce la misma huella, que es lo que hace comparables
// dos máquinas que comparten destino (spec 16).
func TestNewStepFingerprint_EsEstable(t *testing.T) {
	loaded := stepCargado(t)
	assert.Equal(t, huellaDe(t, loaded), huellaDe(t, loaded))
}
