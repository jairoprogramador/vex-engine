package status_test

// Tests de las cuatro reglas de la policy (spec 00 §5.3, corregidos por la 09).
//
// Nacieron como CARACTERIZACIÓN de dos cosas: QUÉ CLAVE usa cada regla contra su
// repositorio, y CUÁNDO escribe. La primera sigue congelada —las claves no son
// uniformes hoy, `time_rule` es la única sin pipeline, y la spec 10 las unifica—.
// La segunda era el defecto: las cuatro reglas escribían dentro de `Evaluate`,
// antes de que el step ejecutara un solo comando.
//
// La spec 09 invierte esas aserciones, que es exactamente para lo que se
// escribieron. Lo que afirman ahora:
//   - `Evaluate` es una CONSULTA: cero escrituras, siempre;
//   - devuelve la evidencia de lo observado, que es lo que otro persistirá
//     después del éxito del step;
//   - un fallo de lectura produce `Undetermined`, distinguible de un cambio.

import (
	"errors"
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

var errRepositorioCaido = errors.New("repositorio caído")

type fakeInstRepo struct {
	callLog
	stored string
	getErr error
	setErr error
}

func (f *fakeInstRepo) Get(idProject, idPipeline, idStep string) (string, error) {
	f.recordGet(joinKey(idProject, idPipeline, idStep))
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.stored, nil
}
func (f *fakeInstRepo) Set(idProject, idPipeline, idStep, fingerprint string) error {
	f.recordSet(joinKey(idProject, idPipeline, idStep))
	if f.setErr != nil {
		return f.setErr
	}
	f.stored = fingerprint
	return nil
}
func (f *fakeInstRepo) Delete(idProject, idPipeline, idStep string) error { return nil }

type fakeCodeRepo struct {
	callLog
	stored string
	getErr error
	setErr error
}

func (f *fakeCodeRepo) Get(idProject, idPipeline, idStep string) (string, error) {
	f.recordGet(joinKey(idProject, idPipeline, idStep))
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.stored, nil
}
func (f *fakeCodeRepo) Set(idProject, idPipeline, idStep, fingerprint string) error {
	f.recordSet(joinKey(idProject, idPipeline, idStep))
	if f.setErr != nil {
		return f.setErr
	}
	f.stored = fingerprint
	return nil
}
func (f *fakeCodeRepo) Delete(idProject, idPipeline, idStep string) error { return nil }

type fakeVarsRepo struct {
	callLog
	stored string
	getErr error
	setErr error
}

func (f *fakeVarsRepo) Get(idProject, idPipeline, idEnvironment, idStep string) (string, error) {
	f.recordGet(joinKey(idProject, idPipeline, idEnvironment, idStep))
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.stored, nil
}
func (f *fakeVarsRepo) Set(idProject, idPipeline, idEnvironment, idStep, fingerprint string) error {
	f.recordSet(joinKey(idProject, idPipeline, idEnvironment, idStep))
	if f.setErr != nil {
		return f.setErr
	}
	f.stored = fingerprint
	return nil
}
func (f *fakeVarsRepo) Delete(idProject, idPipeline, idEnvironment, idStep string) error { return nil }

type fakeTimeRepo struct {
	callLog
	stored time.Time
	getErr error
	setErr error
}

func (f *fakeTimeRepo) Get(idProject, idEnvironment, idStep string) (time.Time, error) {
	f.recordGet(joinKey(idProject, idEnvironment, idStep))
	if f.getErr != nil {
		return time.Time{}, f.getErr
	}
	return f.stored, nil
}
func (f *fakeTimeRepo) Set(idProject, idEnvironment, idStep string, t time.Time) error {
	f.recordSet(joinKey(idProject, idEnvironment, idStep))
	if f.setErr != nil {
		return f.setErr
	}
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

// unicaEvidencia comprueba que una regla hoja devuelve exactamente una
// observación y la desenvuelve.
func unicaEvidencia(t *testing.T, evidencias []status.Evidence) status.Evidence {
	t.Helper()
	require.Len(t, evidencias, 1, "una regla hoja observa una sola cosa")
	return evidencias[0]
}

// --- 5.3 clave de LECTURA de cada regla, y cero escrituras -----------------

func TestReglas_ClaveDeLecturaYCeroEscrituras(t *testing.T) {
	// La clave sigue congelada; lo que cambió es la columna de la derecha:
	// donde antes se afirmaba «la regla ESCRIBE dentro de Evaluate», ahora se
	// afirma que no escribe nunca (spec 09 §5.1).

	t.Run("instructions_pipeline: clave (projectUrl, pipelineUrl, step)", func(t *testing.T) {
		repo := &fakeInstRepo{}
		rule := status.NewInstructionsPipelineRule(repo)

		ctx := baseContext(t)
		ctx[status.InstCurrentParam] = someCommands(t)

		decision, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, status.InstPipelineRuleName, rule.Name())
		assert.True(t, decision.ShouldRun(), "sin estado previo la regla manda ejecutar")
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, stepName)}, repo.gets)
		assert.Empty(t, repo.sets, "evaluar es una consulta: NO escribe")

		evidencia := unicaEvidencia(t, evidencias)
		assert.Equal(t, status.InstPipelineRuleName, evidencia.RuleName)
		assert.NotEmpty(t, evidencia.Current, "la huella observada viaja en la evidencia")
		assert.True(t, evidencia.Changed)
	})

	t.Run("variables_rule: clave (projectUrl, pipelineUrl, environment, step)", func(t *testing.T) {
		repo := &fakeVarsRepo{}
		rule := status.NewVariablesRuleRule(repo)

		ctx := baseContext(t)
		ctx[status.VariablesCurrentParam] = someVariables(t)

		decision, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, status.VariablesRuleName, rule.Name())
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, environment, stepName)}, repo.gets,
			"es la única regla cuya clave lleva environment Y pipeline")
		assert.Empty(t, repo.sets, "evaluar es una consulta: NO escribe")
		assert.Equal(t, status.VariablesRuleName, unicaEvidencia(t, evidencias).RuleName)
	})

	t.Run("code_project_rule: clave (projectUrl, pipelineUrl, step)", func(t *testing.T) {
		repo := &fakeCodeRepo{}
		rule := status.NewCodeProjectRuleRule(repo)

		ctx := baseContext(t)
		ctx[status.ProjectStatusCurrentParam] = "v1:huella-actual"

		decision, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, status.CodeProjectRuleName, rule.Name())
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, []string{joinKey(projectURL, pipelineURL, stepName)}, repo.gets)
		assert.Empty(t, repo.sets, "evaluar es una consulta: NO escribe")

		// Spec 08 §5.3: el prefijo de versión no se pierde por el camino. Una
		// huella sin versión es indistinguible de una huella de otra versión.
		assert.Equal(t, "v1:huella-actual", unicaEvidencia(t, evidencias).Current)
	})

	t.Run("time_rule: clave (projectUrl, environment, step) — SIN pipeline", func(t *testing.T) {
		repo := &fakeTimeRepo{}
		rule := status.NewTimeRule(repo)

		ctx := baseContext(t)
		ctx[status.CurrentTimeParam] = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

		decision, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, status.TimeRuleName, rule.Name())
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, []string{joinKey(projectURL, environment, stepName)}, repo.gets,
			"DIVERGENCIA: es la única clave sin pipelineUrl; dos pipelines del mismo "+
				"proyecto comparten la marca de tiempo. La spec 10 unifica las claves.")
		assert.Empty(t, repo.sets, "evaluar es una consulta: NO escribe")
		assert.Equal(t, status.FormatEvidenceTime(time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)),
			unicaEvidencia(t, evidencias).Current)
	})
}

// --- la consulta es idempotente (spec 09 §7) -------------------------------

func TestReglas_EvaluarDosVecesDaLoMismoYNoEscribeNada(t *testing.T) {
	// ESTE es el test que cambia de color con la spec 09, y el que hace
	// observable la causa raíz de la §1(a): hasta ahora la primera llamada
	// escribía la huella nueva, así que la segunda encontraba otra cosa y
	// respondía `Skip` donde la primera dijo `Run`. La misma pregunta hecha dos
	// veces daba dos respuestas, que es lo que convierte «se evaluó y el proceso
	// murió» en «la corrida siguiente se lo salta».

	t.Run("instructions_pipeline", func(t *testing.T) {
		repo := &fakeInstRepo{}
		rule := status.NewInstructionsPipelineRule(repo)
		ctx := baseContext(t)
		ctx[status.InstCurrentParam] = someCommands(t)

		primera, evidenciaA, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		segunda, evidenciaB, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, primera.Action(), segunda.Action())
		assert.Equal(t, primera.Reason(), segunda.Reason())
		assert.Equal(t, evidenciaA, evidenciaB)
		assert.Empty(t, repo.sets, "cero escrituras después de dos evaluaciones")
	})

	t.Run("variables_rule", func(t *testing.T) {
		repo := &fakeVarsRepo{}
		rule := status.NewVariablesRuleRule(repo)
		ctx := baseContext(t)
		ctx[status.VariablesCurrentParam] = someVariables(t)

		primera, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		segunda, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, primera.Action(), segunda.Action())
		assert.Empty(t, repo.sets)
	})

	t.Run("code_project_rule", func(t *testing.T) {
		repo := &fakeCodeRepo{}
		rule := status.NewCodeProjectRuleRule(repo)
		ctx := baseContext(t)
		ctx[status.ProjectStatusCurrentParam] = "v1:huella-actual"

		primera, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		segunda, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, primera.Action(), segunda.Action())
		assert.Empty(t, repo.sets)
	})

	t.Run("time_rule", func(t *testing.T) {
		repo := &fakeTimeRepo{}
		rule := status.NewTimeRule(repo)
		ctx := baseContext(t)
		ctx[status.CurrentTimeParam] = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

		primera, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		segunda, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.Equal(t, primera.Action(), segunda.Action())
		assert.Empty(t, repo.sets)
	})
}

// --- huella igual ⇒ skip ----------------------------------------------------

func TestReglas_HuellaIgualSalta(t *testing.T) {
	// El estado previo se SIEMBRA en el repositorio, porque ya no lo deja allí la
	// evaluación anterior: escribir es cosa del camino de éxito del step.

	t.Run("instructions_pipeline", func(t *testing.T) {
		repo := &fakeInstRepo{}
		rule := status.NewInstructionsPipelineRule(repo)
		ctx := baseContext(t)
		ctx[status.InstCurrentParam] = someCommands(t)

		_, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		repo.stored = unicaEvidencia(t, evidencias).Current

		segunda, evidenciasB, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.False(t, segunda.ShouldRun())
		assert.Equal(t, "las instrucciones del pipeline no han cambiado", segunda.Reason())
		assert.False(t, unicaEvidencia(t, evidenciasB).Changed)
		assert.Empty(t, repo.sets)
	})

	t.Run("variables_rule", func(t *testing.T) {
		repo := &fakeVarsRepo{}
		rule := status.NewVariablesRuleRule(repo)
		ctx := baseContext(t)
		ctx[status.VariablesCurrentParam] = someVariables(t)

		_, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		repo.stored = unicaEvidencia(t, evidencias).Current

		segunda, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.False(t, segunda.ShouldRun())
		assert.Equal(t, "las variables no han cambiado", segunda.Reason())
		assert.Empty(t, repo.sets)
	})

	t.Run("code_project_rule", func(t *testing.T) {
		repo := &fakeCodeRepo{stored: "v1:huella-actual"}
		rule := status.NewCodeProjectRuleRule(repo)
		ctx := baseContext(t)
		ctx[status.ProjectStatusCurrentParam] = "v1:huella-actual"

		decision, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.False(t, decision.ShouldRun())
		assert.Equal(t, "el código del proyecto no ha cambiado", decision.Reason())
		assert.Empty(t, repo.sets)
	})

	t.Run("time_rule: dentro del TTL de 30 días", func(t *testing.T) {
		ahora := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
		repo := &fakeTimeRepo{stored: ahora.Add(-29 * 24 * time.Hour)}
		rule := status.NewTimeRule(repo)
		ctx := baseContext(t)
		ctx[status.CurrentTimeParam] = ahora

		decision, _, err := rule.Evaluate(ctx)
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

		decision, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.True(t, decision.ShouldRun())
		assert.Empty(t, repo.sets)
	})
}

// --- TimeRule: los mensajes decían lo contrario (spec 09 §5.4) --------------

func TestTimeRule_LosMensajesYaNoEstanInvertidos(t *testing.T) {
	ahora := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	ctx := baseContext(t)
	ctx[status.CurrentTimeParam] = ahora

	t.Run("dentro del TTL dice que NO ha expirado", func(t *testing.T) {
		// Decía «el tiempo de codigo a expirado» justo cuando NO había expirado.
		rule := status.NewTimeRule(&fakeTimeRepo{stored: ahora.Add(-29 * 24 * time.Hour)})

		decision, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.False(t, decision.ShouldRun())
		assert.Equal(t, "no ha expirado el tiempo desde la última ejecución", decision.Reason())
	})

	t.Run("pasado el TTL dice que SÍ ha expirado", func(t *testing.T) {
		rule := status.NewTimeRule(&fakeTimeRepo{stored: ahora.Add(-31 * 24 * time.Hour)})

		decision, _, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		assert.True(t, decision.ShouldRun())
		assert.Equal(t, "ha expirado el tiempo desde la última ejecución", decision.Reason())
	})

	t.Run("la evidencia lleva el instante actual también cuando la regla salta", func(t *testing.T) {
		// Spec 09 §5.4: la marca se refresca cuando el STEP se ejecuta, no cuando
		// esta regla decide ejecutar. Que la evidencia lleve el instante actual
		// aunque la decisión sea `Skip` es lo que hace posible ese refresco: si el
		// step corre por otro motivo, el TTL se reinicia igual.
		rule := status.NewTimeRule(&fakeTimeRepo{stored: ahora.Add(-29 * 24 * time.Hour)})

		decision, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)

		require.False(t, decision.ShouldRun())
		assert.Equal(t, status.FormatEvidenceTime(ahora), unicaEvidencia(t, evidencias).Current)
	})

	t.Run("el instante de la evidencia se recupera intacto", func(t *testing.T) {
		conNanos := time.Date(2026, 8, 4, 12, 34, 56, 123456789, time.UTC)
		recuperado, err := status.ParseEvidenceTime(status.FormatEvidenceTime(conNanos))
		require.NoError(t, err)
		assert.True(t, conNanos.Equal(recuperado))
	})
}

// --- un fallo de lectura NO se disfraza de cambio (spec 09 §5.3) -----------

func TestReglas_FalloDeLecturaEsUndetermined(t *testing.T) {
	// (c) de la spec 09 §1: «no pude leer el estado anterior» y «el código
	// cambió» producían la misma decisión con la misma forma de razón.

	casos := []struct {
		nombre    string
		evaluar   func(status.RuleContext) (status.Decision, []status.Evidence, error)
		completar func(status.RuleContext)
	}{
		{
			nombre:  "instructions_pipeline",
			evaluar: status.NewInstructionsPipelineRule(&fakeInstRepo{getErr: errRepositorioCaido}).Evaluate,
			completar: func(ctx status.RuleContext) {
				ctx[status.InstCurrentParam] = []command.Command{}
			},
		},
		{
			nombre:  "code_project_rule",
			evaluar: status.NewCodeProjectRuleRule(&fakeCodeRepo{getErr: errRepositorioCaido}).Evaluate,
			completar: func(ctx status.RuleContext) {
				ctx[status.ProjectStatusCurrentParam] = "v1:huella-actual"
			},
		},
		{
			nombre:  "time_rule",
			evaluar: status.NewTimeRule(&fakeTimeRepo{getErr: errRepositorioCaido}).Evaluate,
			completar: func(ctx status.RuleContext) {
				ctx[status.CurrentTimeParam] = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
			},
		},
	}

	for _, tc := range casos {
		t.Run(tc.nombre, func(t *testing.T) {
			ctx := baseContext(t)
			tc.completar(ctx)

			decision, evidencias, err := tc.evaluar(ctx)

			require.ErrorIs(t, err, errRepositorioCaido)
			assert.True(t, decision.ShouldRun(), "fail-open: ante la duda, ejecutar")
			assert.True(t, decision.IsUndetermined(),
				"pero se ejecuta por no saber, no por saber que cambió")
			assert.NotContains(t, decision.Reason(), "cambi")

			// Lo observado AHORA sí se pudo calcular, y viaja: si el step termina
			// bien, esa huella queda escrita aunque no se pudiera leer la anterior.
			evidencia := unicaEvidencia(t, evidencias)
			assert.NotEmpty(t, evidencia.Current)
			assert.True(t, evidencia.Persistable())
			assert.False(t, evidencia.Changed,
				"con Undetermined no se afirma lo que no se observó")
		})
	}
}

// --- parámetro ausente ⇒ Undetermined + error ------------------------------

func TestReglas_ParametroAusenteEsUndetermined(t *testing.T) {
	cases := []struct {
		name string
		eval func(status.RuleContext) (status.Decision, []status.Evidence, error)
	}{
		{"instructions_pipeline", status.NewInstructionsPipelineRule(&fakeInstRepo{}).Evaluate},
		{"variables_rule", status.NewVariablesRuleRule(&fakeVarsRepo{}).Evaluate},
		{"code_project_rule", status.NewCodeProjectRuleRule(&fakeCodeRepo{}).Evaluate},
		{"time_rule", status.NewTimeRule(&fakeTimeRepo{}).Evaluate},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision, evidencias, err := tc.eval(status.RuleContext{})

			require.Error(t, err)
			assert.True(t, decision.ShouldRun(), "ante la duda, ejecutar")
			assert.True(t, decision.IsUndetermined())
			assert.False(t, unicaEvidencia(t, evidencias).Persistable(),
				"no se observó nada, así que no hay nada que guardar")
		})
	}
}

// --- qué entra y qué no entra en el material de cada huella ---------------

func TestVariablesRule_MaterialDeLaHuella(t *testing.T) {
	evaluar := func(t *testing.T, mutar func(vars *command.ExecutionVariableMap)) string {
		t.Helper()
		rule := status.NewVariablesRuleRule(&fakeVarsRepo{})

		vars := someVariables(t)
		if mutar != nil {
			mutar(vars)
		}
		ctx := baseContext(t)
		ctx[status.VariablesCurrentParam] = vars

		// La huella se lee de la evidencia, no del repositorio: ya no queda
		// escrita en ninguna parte al evaluar.
		_, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		return unicaEvidencia(t, evidencias).Current
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
		rule := status.NewVariablesRuleRule(&fakeVarsRepo{})

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

		_, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		assert.Equal(t, base, unicaEvidencia(t, evidencias).Current)
	})
}

func TestInstructionsRule_MaterialDeLaHuella(t *testing.T) {
	evaluar := func(t *testing.T, cmds []command.Command) string {
		t.Helper()
		rule := status.NewInstructionsPipelineRule(&fakeInstRepo{})
		ctx := baseContext(t)
		ctx[status.InstCurrentParam] = cmds

		_, evidencias, err := rule.Evaluate(ctx)
		require.NoError(t, err)
		return unicaEvidencia(t, evidencias).Current
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
