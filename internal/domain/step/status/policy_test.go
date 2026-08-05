package status_test

// Tests de CARACTERIZACIÓN de la Policy y del PolicyBuilder (spec 00 §5.3).
//
// El caso central es `cero reglas ⇒ DecisionSkip`: un step cuyo nombre no sea
// test/supply/package/deploy recibe una policy vacía y NUNCA se ejecuta, en
// silencio. Es el bug que corrige la spec 05; congelarlo aquí hace que la
// corrección aparezca como un diff rojo en vez de como un cambio de humor.

import (
	"errors"
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/step/status"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- regla de juguete ------------------------------------------------------

type stubRule struct {
	name     string
	decision status.Decision
	err      error
	calls    *int
	orden    *[]string // registra el nombre de cada regla evaluada, en orden
}

func (s stubRule) Name() string { return s.name }

func (s stubRule) Evaluate(_ status.RuleContext) (status.Decision, error) {
	if s.calls != nil {
		*s.calls++
	}
	if s.orden != nil {
		*s.orden = append(*s.orden, s.name)
	}
	return s.decision, s.err
}

func ruleRun(name, reason string) stubRule {
	return stubRule{name: name, decision: status.DecisionRun(reason)}
}

func ruleSkip(name, reason string) stubRule {
	return stubRule{name: name, decision: status.DecisionSkip(reason)}
}

func ruleErr(name string, err error) stubRule {
	return stubRule{name: name, decision: status.DecisionRun("fallback"), err: err}
}

// --- Policy.Evaluate -------------------------------------------------------

func TestPolicy_Evaluate(t *testing.T) {
	t.Run("CERO reglas ⇒ skip", func(t *testing.T) {
		// DEFECTO VIVO (spec 05): un step sin reglas se salta para siempre.
		// Este test afirma el comportamiento ACTUAL, no el deseado.
		policy := status.NewPolicy("notify")

		decision, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.False(t, decision.ShouldRun(),
			"hoy un step con nombre desconocido nunca se ejecuta")
		assert.Equal(t, "all rules passed", decision.Reason())
	})

	t.Run("todas las reglas dicen skip ⇒ skip", func(t *testing.T) {
		policy := status.NewPolicy("test",
			ruleSkip("a", "sin cambios en a"),
			ruleSkip("b", "sin cambios en b"),
		)

		decision, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.False(t, decision.ShouldRun())
		assert.Equal(t, "all rules passed", decision.Reason())
	})

	t.Run("una sola regla dice run ⇒ run", func(t *testing.T) {
		policy := status.NewPolicy("test",
			ruleSkip("a", "sin cambios en a"),
			ruleRun("b", "cambió b"),
		)

		decision, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, "cambió b", decision.Reason())
	})

	t.Run("los motivos de run se concatenan con '; '", func(t *testing.T) {
		policy := status.NewPolicy("test",
			ruleRun("a", "cambió a"),
			ruleSkip("b", "sin cambios en b"),
			ruleRun("c", "cambió c"),
		)

		decision, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.Equal(t, "cambió a; cambió c", decision.Reason())
	})

	t.Run("una regla que falla NO corta la evaluación de las demás", func(t *testing.T) {
		llamadas := 0
		siguiente := ruleSkip("b", "sin cambios en b")
		siguiente.calls = &llamadas

		policy := status.NewPolicy("test",
			ruleErr("a", errors.New("repositorio caído")),
			siguiente,
		)

		decision, err := policy.Evaluate(status.RuleContext{})

		require.Error(t, err, "el error se propaga junto a la decisión")
		assert.True(t, decision.ShouldRun(), "un fallo fuerza ejecutar")
		assert.Equal(t, "[a] error: repositorio caído", decision.Reason())
		assert.Equal(t, 1, llamadas, "la regla siguiente se evalúa igualmente")
	})

	t.Run("la decisión de una regla que falla se DESCARTA", func(t *testing.T) {
		// La rama de error hace `continue`: `result` nunca se consulta, ni
		// siquiera para leer su Reason. Sólo llega el texto del error.
		fallo := stubRule{
			name:     "a",
			decision: status.DecisionSkip("este motivo no se usa jamás"),
			err:      errors.New("boom"),
		}
		policy := status.NewPolicy("test", fallo)

		decision, _ := policy.Evaluate(status.RuleContext{})

		assert.Equal(t, "[a] error: boom", decision.Reason())
	})

	t.Run("Policy es a su vez una Rule y se puede anidar", func(t *testing.T) {
		var anidada status.Rule = status.NewPolicy("interna", ruleRun("a", "cambió a"))
		externa := status.NewPolicy("externa", anidada)

		decision, err := externa.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, "cambió a", decision.Reason())
	})

	t.Run("AddRule añade al final", func(t *testing.T) {
		policy := status.NewPolicy("test", ruleRun("a", "cambió a"))
		policy.AddRule(ruleRun("b", "cambió b"))

		decision, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.Equal(t, "cambió a; cambió b", decision.Reason())
	})

	t.Run("Name devuelve el nombre del step", func(t *testing.T) {
		assert.Equal(t, "deploy", status.NewPolicy("deploy").Name())
	})
}

// --- PolicyBuilder: qué reglas recibe cada step ---------------------------

func TestPolicyBuilder_Build(t *testing.T) {
	var evaluadas []string

	registry := status.NewRuleRegistry()
	for _, name := range []string{
		status.InstPipelineRuleName,
		status.VariablesRuleName,
		status.CodeProjectRuleName,
		status.TimeRuleName,
	} {
		rule := ruleSkip(name, "sin cambios")
		rule.orden = &evaluadas
		registry.Register(rule)
	}

	cases := []struct {
		step string
		want []string
		nota string
	}{
		{step: "test", want: status.PolicyTestRulesNames},
		{step: "supply", want: status.PolicySupplyRulesNames},
		{step: "package", want: status.PolicyPackageRulesNames},
		{step: "deploy", want: status.PolicyDeployRulesNames},
		{
			step: "notify",
			want: nil,
			nota: "DEFECTO VIVO (spec 05): cualquier otro nombre recibe CERO reglas " +
				"y una policy vacía siempre salta, así que el step nunca se ejecuta",
		},
		{
			step: "01-test",
			want: nil,
			nota: "el switch compara contra el nombre SIN el prefijo NN-",
		},
		{
			step: "Test",
			want: nil,
			nota: "el switch es sensible a mayúsculas",
		},
	}

	for _, tc := range cases {
		t.Run(tc.step, func(t *testing.T) {
			evaluadas = nil

			policy, err := status.NewPolicyBuilder(registry).Build(tc.step)
			require.NoError(t, err)
			require.NotNil(t, policy)
			assert.Equal(t, tc.step, policy.Name())

			decision, err := policy.Evaluate(status.RuleContext{})
			require.NoError(t, err)

			assert.Equal(t, tc.want, evaluadas,
				"reglas evaluadas por el step %q. %s", tc.step, tc.nota)
			assert.False(t, decision.ShouldRun(),
				"con reglas de juguete que siempre pasan, la policy siempre salta")
			assert.Equal(t, "all rules passed", decision.Reason())
		})
	}

	t.Run("error si una regla declarada no está registrada", func(t *testing.T) {
		_, err := status.NewPolicyBuilder(status.NewRuleRegistry()).Build("test")
		require.Error(t, err)
	})
}
