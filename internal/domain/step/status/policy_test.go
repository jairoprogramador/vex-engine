package status_test

// Tests de la Policy y del PolicyBuilder (spec 00 §5.3, corregidos por la 05).
//
// Nacieron como CARACTERIZACIÓN: el caso central era `cero reglas ⇒
// DecisionSkip`, que es el defecto por el que un step cuyo nombre no fuera
// test/supply/package/deploy nunca se ejecutaba, en silencio. La spec 05 invierte
// las dos aserciones que lo congelaban —aquí abajo y en la tabla del builder—,
// que era exactamente para lo que se escribieron.
//
// Lo que afirman ahora:
//   - cero reglas ⇒ DecisionRun. Sin evidencia no se concluye «nada cambió».
//   - un step desconocido ⇒ `Build` devuelve error nombrándolo.
//   - los cuatro steps conocidos siguen recibiendo exactamente las mismas
//     reglas, en el mismo orden.
//
// La spec 09 añade el tercer estado. Lo que se afirma de él aquí:
//   - una regla que no pudo averiguar si algo cambió produce `Undetermined`, y
//     su motivo es DISTINGUIBLE del de un cambio real;
//   - `Undetermined` sigue ejecutando —fail-open— pero deja de ser silencioso;
//   - la guarda de cero reglas NO cae en el tercer estado: sigue siendo `Run`
//     con `ReasonNoRules`. «No hay nada que averiguar» es una respuesta
//     determinada, no un repositorio caído.

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
	evidence []status.Evidence
	err      error
	calls    *int
	orden    *[]string // registra el nombre de cada regla evaluada, en orden
}

func (s stubRule) Name() string { return s.name }

func (s stubRule) Evaluate(_ status.RuleContext) (status.Decision, []status.Evidence, error) {
	if s.calls != nil {
		*s.calls++
	}
	if s.orden != nil {
		*s.orden = append(*s.orden, s.name)
	}
	return s.decision, s.evidence, s.err
}

func ruleRun(name, reason string) stubRule {
	return stubRule{
		name:     name,
		decision: status.DecisionRun(reason),
		evidence: []status.Evidence{status.NewEvidence(name, "ahora", "antes", true)},
	}
}

func ruleSkip(name, reason string) stubRule {
	return stubRule{
		name:     name,
		decision: status.DecisionSkip(reason),
		evidence: []status.Evidence{status.NewEvidence(name, "ahora", "ahora", false)},
	}
}

// ruleUndetermined es la regla que no pudo averiguarlo: devuelve el tercer
// estado y el error de infraestructura que lo causó.
func ruleUndetermined(name, reason string, err error) stubRule {
	return stubRule{
		name:     name,
		decision: status.DecisionUndetermined(reason),
		evidence: []status.Evidence{status.NoEvidence(name)},
		err:      err,
	}
}

// --- Policy.Evaluate -------------------------------------------------------

func TestPolicy_Evaluate(t *testing.T) {
	t.Run("CERO reglas ⇒ run", func(t *testing.T) {
		// Spec 05 §5.1. Este test decía lo contrario hasta la 05: era la
		// caracterización del defecto. Un conjunto vacío de comprobaciones no es
		// evidencia de que nada cambió, y la guarda vive en `Policy` —no solo en
		// el builder— para que no reaparezca por otra vía de construcción.
		policy := status.NewPolicy("notify")

		decision, _, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.True(t, decision.ShouldRun(),
			"sin evidencia no se concluye «sin cambios»: ante la duda, ejecutar")
		assert.Equal(t, status.ReasonNoRules, decision.Reason(),
			"y el motivo dice por qué, no «all rules passed»")
	})

	t.Run("todas las reglas dicen skip ⇒ skip", func(t *testing.T) {
		policy := status.NewPolicy("test",
			ruleSkip("a", "sin cambios en a"),
			ruleSkip("b", "sin cambios en b"),
		)

		decision, _, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.False(t, decision.ShouldRun())
		assert.Equal(t, "all rules passed", decision.Reason())
	})

	t.Run("una sola regla dice run ⇒ run", func(t *testing.T) {
		policy := status.NewPolicy("test",
			ruleSkip("a", "sin cambios en a"),
			ruleRun("b", "cambió b"),
		)

		decision, _, err := policy.Evaluate(status.RuleContext{})

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

		decision, _, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.Equal(t, "cambió a; cambió c", decision.Reason())
	})

	t.Run("una regla que falla NO corta la evaluación de las demás", func(t *testing.T) {
		llamadas := 0
		siguiente := ruleSkip("b", "sin cambios en b")
		siguiente.calls = &llamadas

		policy := status.NewPolicy("test",
			ruleUndetermined("a", "no se pudo leer el estado anterior", errors.New("repositorio caído")),
			siguiente,
		)

		decision, _, err := policy.Evaluate(status.RuleContext{})

		require.Error(t, err, "el error se propaga junto a la decisión")
		assert.True(t, decision.ShouldRun(), "fail-open: ante la duda, ejecutar")
		assert.Equal(t, 1, llamadas, "la regla siguiente se evalúa igualmente")
	})

	// --- el tercer estado (spec 09 §5.3) -----------------------------------

	t.Run("solo indeterminadas ⇒ Undetermined, y lo dice con esas palabras", func(t *testing.T) {
		// (c) de la spec 09 §1: hasta ahora «no pude leer el estado anterior» y
		// «el código cambió» producían la misma decisión con la misma forma de
		// razón. El fail-open se conserva; lo que deja de ser es silencioso.
		policy := status.NewPolicy("test",
			ruleSkip("a", "sin cambios en a"),
			ruleUndetermined("b", "no se pudo leer el estado anterior", errors.New("repositorio caído")),
		)

		decision, _, err := policy.Evaluate(status.RuleContext{})

		require.Error(t, err, "el error de infraestructura se propaga, no se traga")
		assert.True(t, decision.ShouldRun(), "fail-open")
		assert.True(t, decision.IsUndetermined(),
			"y es un estado propio: ni «sé que cambió» ni «sé que no»")
		assert.Contains(t, decision.Reason(), status.ReasonUndetermined)
		assert.Contains(t, decision.Reason(), "[b]", "el motivo nombra a la regla que no pudo")
	})

	t.Run("un cambio REAL gana al indeterminado, que igual se reporta", func(t *testing.T) {
		policy := status.NewPolicy("test",
			ruleRun("a", "cambió a"),
			ruleUndetermined("b", "no se pudo leer el estado anterior", errors.New("repositorio caído")),
		)

		decision, _, _ := policy.Evaluate(status.RuleContext{})

		assert.False(t, decision.IsUndetermined(),
			"si alguna comprobación SABE que algo cambió, ese es el motivo del step")
		assert.Contains(t, decision.Reason(), "cambió a")
		assert.Contains(t, decision.Reason(), status.ReasonUndetermined,
			"pero el caché roto sigue siendo visible")
	})

	t.Run("un cambio real y un indeterminado son distinguibles entre sí", func(t *testing.T) {
		// La aserción de (c): las dos razones no pueden tener la misma forma.
		cambio, _, _ := status.NewPolicy("test", ruleRun("a", "cambió a")).
			Evaluate(status.RuleContext{})
		duda, _, _ := status.NewPolicy("test",
			ruleUndetermined("a", "no se pudo leer el estado anterior", errors.New("boom"))).
			Evaluate(status.RuleContext{})

		require.True(t, cambio.ShouldRun())
		require.True(t, duda.ShouldRun())
		assert.NotEqual(t, cambio.Action(), duda.Action())
		assert.NotContains(t, cambio.Reason(), status.ReasonUndetermined)
	})

	t.Run("CERO reglas NO es Undetermined", func(t *testing.T) {
		// Spec 09 §5.3: la guarda de la 05 sobrevive al tercer estado. «No hay
		// nada que averiguar» es una respuesta determinada; meterla en el tercer
		// estado la haría indistinguible de un repositorio caído.
		decision, _, err := status.NewPolicy("notify").Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.False(t, decision.IsUndetermined())
		assert.Equal(t, status.ReasonNoRules, decision.Reason())
	})

	// --- la consulta no escribe (spec 09 §3) --------------------------------

	t.Run("Evaluate devuelve la evidencia de TODAS las reglas", func(t *testing.T) {
		// La evidencia es lo que permite que otro —el camino de éxito del step—
		// decida cuándo persistir la observación.
		policy := status.NewPolicy("test",
			ruleSkip("a", "sin cambios en a"),
			ruleRun("b", "cambió b"),
		)

		_, evidencias, err := policy.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		require.Len(t, evidencias, 2)
		assert.Equal(t, "a", evidencias[0].RuleName)
		assert.False(t, evidencias[0].Changed)
		assert.Equal(t, "b", evidencias[1].RuleName)
		assert.True(t, evidencias[1].Changed)
	})

	t.Run("la evidencia de una regla que no observó nada no es persistible", func(t *testing.T) {
		policy := status.NewPolicy("test",
			ruleUndetermined("a", "no se pudo leer el estado anterior", errors.New("boom")),
		)

		_, evidencias, _ := policy.Evaluate(status.RuleContext{})

		require.Len(t, evidencias, 1)
		assert.False(t, evidencias[0].Persistable(),
			"sin valor observado no hay nada que guardar, y guardar el vacío sería peor")
	})

	t.Run("Policy es a su vez una Rule y se puede anidar", func(t *testing.T) {
		var anidada status.Rule = status.NewPolicy("interna", ruleRun("a", "cambió a"))
		externa := status.NewPolicy("externa", anidada)

		decision, _, err := externa.Evaluate(status.RuleContext{})

		require.NoError(t, err)
		assert.True(t, decision.ShouldRun())
		assert.Equal(t, "cambió a", decision.Reason())
	})

	t.Run("AddRule añade al final", func(t *testing.T) {
		policy := status.NewPolicy("test", ruleRun("a", "cambió a"))
		policy.AddRule(ruleRun("b", "cambió b"))

		decision, _, err := policy.Evaluate(status.RuleContext{})

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

	// Los cuatro steps conocidos: la spec 05 no toca qué reglas recibe cada uno,
	// así que esta tabla es la red que lo demuestra.
	conocidos := []struct {
		step string
		want []string
	}{
		{step: "test", want: status.PolicyTestRulesNames},
		{step: "supply", want: status.PolicySupplyRulesNames},
		{step: "package", want: status.PolicyPackageRulesNames},
		{step: "deploy", want: status.PolicyDeployRulesNames},
	}

	for _, tc := range conocidos {
		t.Run(tc.step, func(t *testing.T) {
			evaluadas = nil

			policy, err := status.NewPolicyBuilder(registry).Build(tc.step)
			require.NoError(t, err)
			require.NotNil(t, policy)
			assert.Equal(t, tc.step, policy.Name())

			decision, _, err := policy.Evaluate(status.RuleContext{})
			require.NoError(t, err)

			assert.Equal(t, tc.want, evaluadas,
				"reglas evaluadas por el step %q, en orden", tc.step)
			assert.False(t, decision.ShouldRun(),
				"con reglas de juguete que siempre pasan, la policy salta")
			assert.Equal(t, status.ReasonAllRulesPassed, decision.Reason())
		})
	}

	// Un step que el motor no conoce. Hasta la spec 05 los tres casos de abajo
	// devolvían una policy vacía SIN error, y esa policy se saltaba siempre: el
	// step no se ejecutaba jamás, con exit code 0.
	desconocidos := []struct {
		step string
		nota string
	}{
		{
			step: "notify",
			nota: "un quinto step legítimo: el motor no sabe qué comprobar y lo dice",
		},
		{
			step: "01-test",
			nota: "el switch compara contra el nombre SIN el prefijo NN-",
		},
		{
			step: "Test",
			nota: "el switch es sensible a mayúsculas",
		},
		{
			step: "",
			nota: "el nombre vacío tampoco cae en un default silencioso",
		},
	}

	for _, tc := range desconocidos {
		t.Run("desconocido/"+tc.step, func(t *testing.T) {
			evaluadas = nil

			policy, err := status.NewPolicyBuilder(registry).Build(tc.step)

			require.Error(t, err, "%s", tc.nota)
			assert.Nil(t, policy, "no se devuelve una policy a medias")
			assert.Contains(t, err.Error(), tc.step, "el error nombra al step")
			for _, conocido := range status.KnownSteps {
				assert.Contains(t, err.Error(), conocido,
					"y enumera los pasos conocidos, que es lo accionable")
			}
			assert.Empty(t, evaluadas, "no se evaluó ninguna regla")
		})
	}

	t.Run("error si una regla declarada no está registrada", func(t *testing.T) {
		_, err := status.NewPolicyBuilder(status.NewRuleRegistry()).Build("test")
		require.Error(t, err)
	})
}
