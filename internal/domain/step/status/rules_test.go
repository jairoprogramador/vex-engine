package status_test

// Tests de CARACTERIZACIÓN de las cuatro reglas de la policy (spec 00 §5.3).
//
// Lo que se congela aquí es, para cada regla: QUÉ CLAVE usa contra su
// repositorio y CUÁNDO escribe. Las claves no son uniformes hoy —
// `time_rule` es la única que no lleva pipeline— y la spec 10 las unifica.
// Que esa unificación aparezca como un diff rojo en este archivo es el objetivo.
//
// También queda congelado que las cuatro reglas ESCRIBEN dentro de `Evaluate`:
// decidir y persistir están fundidos. La spec 09 lo separa.

import (
	"strings"
	"testing"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/step/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- dobles de repositorio que registran las llamadas ----------------------

type callLog struct {
	gets []string
	sets []string
}

func (c *callLog) recordGet(key string) { c.gets = append(c.gets, key) }
func (c *callLog) recordSet(key string) { c.sets = append(c.sets, key) }

func joinKey(parts ...string) string { return strings.Join(parts, "|") }

type fakeInstRepo struct {
	callLog
	stored string
}

func (f *fakeInstRepo) Get(idProject, idPipeline, idStep string) (string, error) {
	f.recordGet(joinKey(idProject, idPipeline, idStep))
	return f.stored, nil
}
func (f *fakeInstRepo) Set(idProject, idPipeline, idStep, fingerprint string) error {
	f.recordSet(joinKey(idProject, idPipeline, idStep))
	f.stored = fingerprint
	return nil
}
func (f *fakeInstRepo) Delete(idProject, idPipeline, idStep string) error { return nil }

type fakeCodeRepo struct {
	callLog
	stored string
}

func (f *fakeCodeRepo) Get(idProject, idPipeline, idStep string) (string, error) {
	f.recordGet(joinKey(idProject, idPipeline, idStep))
	return f.stored, nil
}
func (f *fakeCodeRepo) Set(idProject, idPipeline, idStep, fingerprint string) error {
	f.recordSet(joinKey(idProject, idPipeline, idStep))
	f.stored = fingerprint
	return nil
}
func (f *fakeCodeRepo) Delete(idProject, idPipeline, idStep string) error { return nil }

type fakeVarsRepo struct {
	callLog
	stored string
}

func (f *fakeVarsRepo) Get(idProject, idPipeline, idEnvironment, idStep string) (string, error) {
	f.recordGet(joinKey(idProject, idPipeline, idEnvironment, idStep))
	return f.stored, nil
}
func (f *fakeVarsRepo) Set(idProject, idPipeline, idEnvironment, idStep, fingerprint string) error {
	f.recordSet(joinKey(idProject, idPipeline, idEnvironment, idStep))
	f.stored = fingerprint
	return nil
}
func (f *fakeVarsRepo) Delete(idProject, idPipeline, idEnvironment, idStep string) error { return nil }

type fakeTimeRepo struct {
	callLog
	stored time.Time
}

func (f *fakeTimeRepo) Get(idProject, idEnvironment, idStep string) (time.Time, error) {
	f.recordGet(joinKey(idProject, idEnvironment, idStep))
	return f.stored, nil
}
func (f *fakeTimeRepo) Set(idProject, idEnvironment, idStep string, t time.Time) error {
	f.recordSet(joinKey(idProject, idEnvironment, idStep))
	f.stored = t
	return nil
}
func (f *fakeTimeRepo) Delete(idProject, idEnvironment, idStep string) error { return nil }

// --- material de prueba ----------------------------------------------------

const (
	projectURL  = "https://github.com/org/proyecto"
	pipelineURL = "https://github.com/org/pipeline"
	environment = "prod"
	stepName    = "test"
)

func baseContext(t *testing.T) status.RuleContext {
	t.Helper()
	return status.RuleContext{
		status.ProjectUrlParam:  projectURL,
		status.PipelineUrlParam: pipelineURL,
		status.EnvironmentParam: environment,
		status.StepParam:        stepName,
	}
}

func someCommands(t *testing.T) []command.Command {
	t.Helper()
	output, err := command.NewCommandOutput("artefacto", `version: (\S+)`)
	require.NoError(t, err)

	cmd, err := command.NewCommand("compilar", "mvn package",
		command.WithWorkdir("app"),
		command.WithTemplateFiles([]command.CommandTemplatePath{
			command.NewCommandTemplatePath("app/pom.xml"),
		}),
		command.WithOutputs([]command.CommandOutput{output}),
	)
	require.NoError(t, err)
	return []command.Command{cmd}
}

func someVariables(t *testing.T) *command.ExecutionVariableMap {
	t.Helper()
	vars := command.NewExecutionVariableMap()
	for _, v := range []struct {
		name, value string
		shared      bool
	}{
		{"region", "us-east-1", false},
		{"replicas", "3", false},
		{"bucket", "artefactos", true},
	} {
		variable, err := command.NewVariable(v.name, v.value, v.shared)
		require.NoError(t, err)
		vars.Add(variable)
	}
	return vars
}

// --- 5.3 clave y momento de escritura de cada regla ------------------------

func TestReglas_ClaveDelRepositorioYMomentoDeEscritura(t *testing.T) {
	t.Run("instructions_pipeline: clave (projectUrl, pipelineUrl, step)", func(t *testing.T) {
		repo := &fakeInstRepo{}
		rule := status.NewInstructionsPipelineRule(repo)

		ctx := baseContext(t)
		ctx[status.InstCurrentParam] = someCommands(t)

		decision, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, status.InstPipelineRuleName, rule.Name())
		assert.True(t, decision.ShouldRun(), "sin estado previo la regla manda ejecutar")
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, stepName)}, repo.gets)
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, stepName)}, repo.sets,
			"la regla ESCRIBE dentro de Evaluate")
	})

	t.Run("variables_rule: clave (projectUrl, pipelineUrl, environment, step)", func(t *testing.T) {
		repo := &fakeVarsRepo{}
		rule := status.NewVariablesRuleRule(repo)

		ctx := baseContext(t)
		ctx[status.VariablesCurrentParam] = someVariables(t)

		decision, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, status.VariablesRuleName, rule.Name())
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, environment, stepName)}, repo.gets,
			"es la única regla cuya clave lleva environment Y pipeline")
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, environment, stepName)}, repo.sets,
			"la regla ESCRIBE dentro de Evaluate")
	})

	t.Run("code_project_rule: clave (projectUrl, pipelineUrl, step)", func(t *testing.T) {
		repo := &fakeCodeRepo{}
		rule := status.NewCodeProjectRuleRule(repo)

		ctx := baseContext(t)
		ctx[status.ProjectStatusCurrentParam] = "huella-actual"

		decision, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, status.CodeProjectRuleName, rule.Name())
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, stepName)}, repo.gets)
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, stepName)}, repo.sets,
			"la regla ESCRIBE dentro de Evaluate")
	})

	t.Run("time_rule: clave (projectUrl, environment, step) — SIN pipeline", func(t *testing.T) {
		repo := &fakeTimeRepo{}
		rule := status.NewTimeRule(repo)

		ctx := baseContext(t)
		ctx[status.CurrentTimeParam] = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

		decision, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, status.TimeRuleName, rule.Name())
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, []string{joinKey(projectURL, environment, stepName)}, repo.gets,
			"DIVERGENCIA: es la única clave sin pipelineUrl; dos pipelines del mismo "+
				"proyecto comparten la marca de tiempo. La spec 10 unifica las claves.")
		assert.Equal(t, []string{joinKey(projectURL, environment, stepName)}, repo.sets,
			"la regla ESCRIBE dentro de Evaluate")
	})
}

// --- huella igual ⇒ skip y NO se escribe -----------------------------------

func TestReglas_HuellaIgualSaltaYNoEscribe(t *testing.T) {
	t.Run("instructions_pipeline", func(t *testing.T) {
		repo := &fakeInstRepo{}
		rule := status.NewInstructionsPipelineRule(repo)
		ctx := baseContext(t)
		ctx[status.InstCurrentParam] = someCommands(t)

		primera, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		require.True(t, primera.ShouldRun())

		segunda, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.False(t, segunda.ShouldRun())
		assert.Equal(t, "las instrucciones del pipeline no ha cambiado", segunda.Reason())
		assert.Len(t, repo.sets, 1, "la segunda evaluación no vuelve a escribir")
	})

	t.Run("variables_rule", func(t *testing.T) {
		repo := &fakeVarsRepo{}
		rule := status.NewVariablesRuleRule(repo)
		ctx := baseContext(t)
		ctx[status.VariablesCurrentParam] = someVariables(t)

		_, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		segunda, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.False(t, segunda.ShouldRun())
		assert.Equal(t, "las variables no han cambiado", segunda.Reason())
		assert.Len(t, repo.sets, 1)
	})

	t.Run("code_project_rule", func(t *testing.T) {
		repo := &fakeCodeRepo{stored: "huella-actual"}
		rule := status.NewCodeProjectRuleRule(repo)
		ctx := baseContext(t)
		ctx[status.ProjectStatusCurrentParam] = "huella-actual"

		decision, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.False(t, decision.ShouldRun())
		assert.Equal(t, "el código del proyecto no ha cambiado", decision.Reason())
		assert.Empty(t, repo.sets, "si no cambió, no se escribe")
	})

	t.Run("time_rule: dentro del TTL de 30 días", func(t *testing.T) {
		ahora := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
		repo := &fakeTimeRepo{stored: ahora.Add(-29 * 24 * time.Hour)}
		rule := status.NewTimeRule(repo)
		ctx := baseContext(t)
		ctx[status.CurrentTimeParam] = ahora

		decision, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.False(t, decision.ShouldRun())
		assert.Empty(t, repo.sets)
	})

	t.Run("time_rule: pasado el TTL de 30 días", func(t *testing.T) {
		ahora := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
		repo := &fakeTimeRepo{stored: ahora.Add(-31 * 24 * time.Hour)}
		rule := status.NewTimeRule(repo)
		ctx := baseContext(t)
		ctx[status.CurrentTimeParam] = ahora

		decision, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.True(t, decision.ShouldRun())
		assert.Len(t, repo.sets, 1)
	})
}

// --- qué entra y qué no entra en el material de cada huella ---------------

func TestVariablesRule_MaterialDeLaHuella(t *testing.T) {
	evaluar := func(t *testing.T, mutar func(vars *command.ExecutionVariableMap)) string {
		t.Helper()
		repo := &fakeVarsRepo{}
		rule := status.NewVariablesRuleRule(repo)

		vars := someVariables(t)
		if mutar != nil {
			mutar(vars)
		}
		ctx := baseContext(t)
		ctx[status.VariablesCurrentParam] = vars

		_, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		return repo.stored
	}

	añadir := func(name, value string, shared bool) func(*command.ExecutionVariableMap) {
		return func(vars *command.ExecutionVariableMap) {
			v, err := command.NewVariable(name, value, shared)
			require.NoError(t, err)
			vars.Add(v)
		}
	}

	base := evaluar(t, nil)

	t.Run("las variables derivadas se excluyen del material", func(t *testing.T) {
		// Se borran antes de calcular la huella: cambian en cada ejecución y
		// harían que ningún step se saltara nunca.
		for _, excluida := range []string{
			command.VarProjectVersion,
			command.VarProjectRevision,
			command.VarProjectRevisionFull,
			command.VarToolName,
			command.VarProjectWorkdir,
			command.VarStepWorkdir,
		} {
			t.Run(excluida, func(t *testing.T) {
				assert.Equal(t, base, evaluar(t, añadir(excluida, "valor-cualquiera", false)))
			})
		}
	})

	t.Run("otras variables del entorno SÍ entran", func(t *testing.T) {
		assert.NotEqual(t, base, evaluar(t, añadir(command.VarEnvironment, "sand", false)))
		assert.NotEqual(t, base, evaluar(t, añadir(command.VarSharedWorkdir, "/tmp/shared", false)))
	})

	t.Run("cambiar un valor cambia la huella", func(t *testing.T) {
		assert.NotEqual(t, base, evaluar(t, añadir("region", "eu-west-1", false)))
	})

	t.Run("cambiar sólo el flag shared cambia la huella", func(t *testing.T) {
		assert.NotEqual(t, base, evaluar(t, añadir("region", "us-east-1", true)))
	})

	t.Run("declarada y vacía no es lo mismo que no declarada", func(t *testing.T) {
		// Spec 03 §5.3: desde que el valor vacío es legítimo, la huella tiene
		// que separar los dos estados. Lo consigue el material canónico, que
		// serializa Quote(value) — y Quote("") es `""`, no ausencia.
		assert.NotEqual(t, base, evaluar(t, añadir("instance_count", "", false)))
	})

	t.Run("el orden de inserción no cambia la huella", func(t *testing.T) {
		repo := &fakeVarsRepo{}
		rule := status.NewVariablesRuleRule(repo)

		vars := command.NewExecutionVariableMap()
		for _, v := range []struct {
			name, value string
			shared      bool
		}{
			{"bucket", "artefactos", true},
			{"replicas", "3", false},
			{"region", "us-east-1", false},
		} {
			variable, err := command.NewVariable(v.name, v.value, v.shared)
			require.NoError(t, err)
			vars.Add(variable)
		}
		ctx := baseContext(t)
		ctx[status.VariablesCurrentParam] = vars

		_, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		assert.Equal(t, base, repo.stored)
	})
}

func TestInstructionsRule_MaterialDeLaHuella(t *testing.T) {
	evaluar := func(t *testing.T, cmds []command.Command) string {
		t.Helper()
		repo := &fakeInstRepo{}
		rule := status.NewInstructionsPipelineRule(repo)
		ctx := baseContext(t)
		ctx[status.InstCurrentParam] = cmds

		_, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		return repo.stored
	}

	base := evaluar(t, someCommands(t))

	t.Run("cambiar el cmd cambia la huella", func(t *testing.T) {
		cmd, err := command.NewCommand("compilar", "mvn verify", command.WithWorkdir("app"))
		require.NoError(t, err)
		assert.NotEqual(t, base, evaluar(t, []command.Command{cmd}))
	})

	t.Run("cambiar el workdir cambia la huella", func(t *testing.T) {
		cmd, err := command.NewCommand("compilar", "mvn package", command.WithWorkdir("otro"))
		require.NoError(t, err)
		assert.NotEqual(t, base, evaluar(t, []command.Command{cmd}))
	})

	t.Run("el flag show NO entra en la huella", func(t *testing.T) {
		conShow := someCommands(t)
		sinShow := someCommands(t)

		cmd, err := command.NewCommand("compilar", "mvn package",
			command.WithWorkdir("app"),
			command.WithTemplateFiles(conShow[0].TemplatePaths()),
			command.WithOutputs(conShow[0].Outputs()),
			command.WithShow(true),
		)
		require.NoError(t, err)

		// DEFECTO VIVO: añadir `show: true` para ver por qué falla un comando
		// no invalida el caché, así que el step se salta y no se imprime nada.
		// SPEC 10 §5.1bis: `show` entra en el material; este caso se borra allí.
		assert.Equal(t, evaluar(t, sinShow), evaluar(t, []command.Command{cmd}),
			"cambiar `show` no invalida el cacheo del step")
	})

	t.Run("el orden de los comandos cambia la huella", func(t *testing.T) {
		primero, err := command.NewCommand("a", "echo a")
		require.NoError(t, err)
		segundo, err := command.NewCommand("b", "echo b")
		require.NoError(t, err)

		assert.NotEqual(t,
			evaluar(t, []command.Command{primero, segundo}),
			evaluar(t, []command.Command{segundo, primero}),
		)
	})

	t.Run("sin comandos la huella es sha256(\"\")", func(t *testing.T) {
		assert.Equal(t,
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			evaluar(t, []command.Command{}),
		)
	})
}

// --- parámetro ausente ⇒ run + error ---------------------------------------

func TestReglas_ParametroAusenteMandaEjecutar(t *testing.T) {
	cases := []struct {
		name string
		eval func(status.RuleContext) (status.Decision, error)
	}{
		{"instructions_pipeline", status.NewInstructionsPipelineRule(&fakeInstRepo{}).Evaluate},
		{"variables_rule", status.NewVariablesRuleRule(&fakeVarsRepo{}).Evaluate},
		{"code_project_rule", status.NewCodeProjectRuleRule(&fakeCodeRepo{}).Evaluate},
		{"time_rule", status.NewTimeRule(&fakeTimeRepo{}).Evaluate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, err := tc.eval(status.RuleContext{})

			require.Error(t, err)
			assert.True(t, decision.ShouldRun(), "ante la duda, ejecutar")
		})
	}
}

// --- RuleRegistry ----------------------------------------------------------

func TestRuleRegistry(t *testing.T) {
	registry := status.NewRuleRegistry()
	registry.Register(status.NewCodeProjectRuleRule(&fakeCodeRepo{}))

	t.Run("devuelve la regla registrada", func(t *testing.T) {
		rule, err := registry.Get(status.CodeProjectRuleName)
		require.NoError(t, err)
		assert.Equal(t, status.CodeProjectRuleName, rule.Name())
	})

	t.Run("error si no está registrada", func(t *testing.T) {
		_, err := registry.Get("no-existe")
		require.Error(t, err)
	})
}
