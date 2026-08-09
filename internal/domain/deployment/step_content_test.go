package deployment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
	"github.com/jairoprogramador/vex-engine/internal/domain/step"
)

func TestStepContent_ComposicionYAccesores(t *testing.T) {
	contenido := stepSupply(t)

	assert.Equal(t, "02-supply", contenido.StepID())
	assert.True(t, contenido.Config().IsDeclared())
	assert.Equal(t, fingerprint.InstructionsVersion, contenido.Instructions().Version())
	assert.False(t, contenido.IsZero())

	parametros := contenido.Parameters()
	require.Len(t, parametros, 2)
	assert.Equal(t, "acr_name", parametros[0].Name(), "los parámetros salen ORDENADOS por nombre")
	assert.Equal(t, "image", parametros[1].Name())
}

func TestStepContent_LosParametrosDevueltosSonUnaCopia(t *testing.T) {
	contenido := stepSupply(t)

	devueltos := contenido.Parameters()
	devueltos[0] = step.VariableDeclaration{}

	assert.Equal(t, "acr_name", contenido.Parameters()[0].Name())
}

func TestStepContent_MaterialInvalido(t *testing.T) {
	instrucciones := huella(t, fingerprint.InstructionsVersion, "11")

	casos := map[string]struct {
		stepID       string
		instructions fingerprint.Fingerprint
		parameters   []step.VariableDeclaration
	}{
		"sin step_id": {
			stepID: "", instructions: instrucciones,
		},
		"un step_id que rompería una ruta": {
			stepID: "steps/01-test", instructions: instrucciones,
		},
		"un step_id que atraviesa directorios": {
			stepID: "..", instructions: instrucciones,
		},
		"sin huella de instrucciones": {
			stepID: "01-test", instructions: fingerprint.Fingerprint{},
		},
		"con una huella que no es la de las instrucciones": {
			// Un `v1:` donde va un `inst-v1:` es material de otra regla: aceptarlo
			// haría que la identidad dependiera de qué huella se coló.
			stepID: "01-test", instructions: huella(t, fingerprint.Version, "11"),
		},
		"con un parámetro sin nombre": {
			stepID: "01-test", instructions: instrucciones,
			parameters: []step.VariableDeclaration{{}},
		},
	}

	for nombre, caso := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := deployment.NewStepContent(
				caso.stepID, step.NoStepConfig(), caso.instructions, caso.parameters)
			require.Error(t, err)
		})
	}
}

func TestStepContent_UnParametroDeclaradoDosVecesEsUnError(t *testing.T) {
	// «Gana la última» dependería del orden de lectura del archivo, y con eso la
	// identidad dejaría de ser reproducible entre implementaciones.
	uno, err := step.NewLiteralDeclaration("acr_name", "uno")
	require.NoError(t, err)
	otro, err := step.NewLiteralDeclaration("acr_name", "otro")
	require.NoError(t, err)

	_, err = deployment.NewStepContent(
		"01-test",
		step.NoStepConfig(),
		huella(t, fingerprint.InstructionsVersion, "11"),
		[]step.VariableDeclaration{uno, otro})
	require.Error(t, err)
}

func TestStepContent_ElAmbitoDeProyectoSeReconoce(t *testing.T) {
	assert.True(t, stepSupply(t).IsProjectScoped())

	// Un step sin `config.yaml` no declara ámbito: no pertenece al sub-bloque
	// independiente del destino, y decir que sí sería inventarle un ámbito.
	assert.False(t, stepTest(t).IsProjectScoped())

	reglas, err := step.NewRuleSet(step.NewDefaultStateChangedRule())
	require.NoError(t, err)
	config, err := step.NewStepConfig(step.NewEnvironmentScope(), reglas)
	require.NoError(t, err)
	deAmbiente, err := deployment.NewStepContent(
		"03-deploy", config, huella(t, fingerprint.InstructionsVersion, "33"), nil)
	require.NoError(t, err)

	assert.False(t, deAmbiente.IsProjectScoped())
}

func TestStepContent_ElValorCeroNoEsMaterial(t *testing.T) {
	var contenido deployment.StepContent
	assert.True(t, contenido.IsZero())
	assert.False(t, contenido.IsProjectScoped())
}

func TestStepContent_LasReglasEntranEnLaIdentidad(t *testing.T) {
	// El `config.yaml` de un step ya no declara sólo el ámbito: declara también
	// sus reglas, y lo que el objeto guarda como declaración del step crece con
	// ellas. Una regla que se pudiera cambiar sin mover la identidad sería una
	// que se puede cambiar sin que el step se re-ejecute.
	conRegla := func(regla step.Rule) deployment.Content {
		reglas, err := step.NewRuleSet(regla)
		require.NoError(t, err)
		config, err := step.NewStepConfig(step.NewEnvironmentScope(), reglas)
		require.NoError(t, err)
		contenido, err := deployment.NewStepContent(
			"01-test", config, huella(t, fingerprint.InstructionsVersion, "11"), nil)
		require.NoError(t, err)

		material := materialBase(t)
		material.steps = []deployment.StepContent{contenido}
		return material.componer(t)
	}

	seisHoras, err := step.NewMaxAgeRule("6h")
	require.NoError(t, err)
	unDia, err := step.NewMaxAgeRule("24h")
	require.NoError(t, err)
	soloPipeline, err := step.NewStateChangedRule([]string{step.StateSourcePipeline})
	require.NoError(t, err)

	assert.False(t, conRegla(seisHoras).ID().Equals(conRegla(unDia).ID()),
		"cambiar la ventana de vigencia es cambiar lo que se pretende hacer")
	assert.False(t, conRegla(soloPipeline).ID().Equals(
		conRegla(step.NewDefaultStateChangedRule()).ID()),
		"dejar de vigilar el código del proyecto es un cambio de contenido")
}

func TestStepContent_ElAmbitoDeclaradoEntraEnLaIdentidad(t *testing.T) {
	conAmbito := func(ambito step.Scope) deployment.Content {
		reglas, err := step.NewRuleSet(step.NewDefaultStateChangedRule())
		require.NoError(t, err)
		config, err := step.NewStepConfig(ambito, reglas)
		require.NoError(t, err)
		contenido, err := deployment.NewStepContent(
			"01-test", config, huella(t, fingerprint.InstructionsVersion, "11"), nil)
		require.NoError(t, err)

		material := materialBase(t)
		material.steps = []deployment.StepContent{contenido}
		return material.componer(t)
	}

	assert.False(t,
		conAmbito(step.NewProjectScope()).ID().Equals(
			conAmbito(step.NewEnvironmentScope()).ID()))
}

func TestStepContent_SinConfigYConConfigVaciaNoSonLoMismo(t *testing.T) {
	// «No hay `config.yaml`» y «hay uno que no declara reglas» son dos hechos
	// distintos con consecuencias distintas, y el material tiene que
	// distinguirlos: el primero no declara ámbito.
	sinConfig := materialBase(t)
	sinConfig.steps = []deployment.StepContent{stepTest(t)}

	config, err := step.NewStepConfig(step.NewEnvironmentScope(), step.EmptyRuleSet())
	require.NoError(t, err)
	conConfigVacia, err := deployment.NewStepContent(
		"01-test", config, huella(t, fingerprint.InstructionsVersion, "11"), nil)
	require.NoError(t, err)

	declarada := materialBase(t)
	declarada.steps = []deployment.StepContent{conConfigVacia}

	assert.False(t, sinConfig.componer(t).ID().Equals(declarada.componer(t).ID()))
}
