package step_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
)

// La forma canónica de una regla es lo que aporta a la IDENTIDAD de un step: la
// consume `deployment.StepContent` (spec 17) y la consumirá `pipe-v1` (spec 27).
// Lo que estos tests fijan es qué mueve la identidad y qué no.

func TestRuleCanonical_StateChangedDistingueSusFuentes(t *testing.T) {
	soloPipeline, err := domStep.NewStateChangedRule([]string{domStep.StateSourcePipeline})
	require.NoError(t, err)
	conProyecto, err := domStep.NewStateChangedRule(
		[]string{domStep.StateSourcePipeline, domStep.StateSourceProject})
	require.NoError(t, err)

	assert.NotEqual(t, soloPipeline.Canonical(), conProyecto.Canonical(),
		"dejar de vigilar el código del proyecto es un cambio de contenido")
	assert.Contains(t, soloPipeline.Canonical(), string(domStep.RuleKindStateChanged))
}

func TestRuleCanonical_LaFormaCortaYLaLargaEquivalenteSonLaMisma(t *testing.T) {
	// `- state_changed` a secas equivale a `[pipeline, project]`. Significan lo
	// mismo, así que tienen que producir la misma cadena: la identidad es del
	// significado, no del azúcar sintáctico.
	larga, err := domStep.NewStateChangedRule(
		[]string{domStep.StateSourcePipeline, domStep.StateSourceProject})
	require.NoError(t, err)

	assert.Equal(t, domStep.NewDefaultStateChangedRule().Canonical(), larga.Canonical())
}

func TestRuleCanonical_ElOrdenDeLasFuentesDeclaradasNoLaMueve(t *testing.T) {
	unOrden, err := domStep.NewStateChangedRule(
		[]string{domStep.StateSourcePipeline, domStep.StateSourceProject})
	require.NoError(t, err)
	otroOrden, err := domStep.NewStateChangedRule(
		[]string{domStep.StateSourceProject, domStep.StateSourcePipeline})
	require.NoError(t, err)

	assert.Equal(t, unOrden.Canonical(), otroOrden.Canonical())
}

func TestRuleCanonical_MaxAgeNormalizaLaUnidad(t *testing.T) {
	// `60m` y `1h` declaran lo mismo. Reescribir la unidad no es un cambio de
	// intención, y por tanto no debe re-ejecutar nada.
	enMinutos, err := domStep.NewMaxAgeRule("60m")
	require.NoError(t, err)
	enHoras, err := domStep.NewMaxAgeRule("1h")
	require.NoError(t, err)
	otraVentana, err := domStep.NewMaxAgeRule("720h")
	require.NoError(t, err)

	assert.Equal(t, enMinutos.Canonical(), enHoras.Canonical())
	assert.NotEqual(t, enHoras.Canonical(), otraVentana.Canonical())
	assert.Contains(t, enHoras.Canonical(), string(domStep.RuleKindMaxAge))
}

func TestRuleSetCanonical_ElConjuntoVacioEsLaCadenaVacia(t *testing.T) {
	assert.Empty(t, domStep.EmptyRuleSet().Canonical())
}

func TestRuleSetCanonical_ElOrdenDECLARADOEntra(t *testing.T) {
	// El orden es el de evaluación, y por tanto el que decide qué motivo se emite
	// cuando dos reglas se cumplen a la vez. Reordenar `rules` cambia lo que el
	// usuario verá, así que cambia el contenido.
	maxAge, err := domStep.NewMaxAgeRule("24h")
	require.NoError(t, err)

	unOrden, err := domStep.NewRuleSet(domStep.NewDefaultStateChangedRule(), maxAge)
	require.NoError(t, err)
	otroOrden, err := domStep.NewRuleSet(maxAge, domStep.NewDefaultStateChangedRule())
	require.NoError(t, err)

	assert.NotEqual(t, unOrden.Canonical(), otroOrden.Canonical())
}

func TestRuleSetCanonical_DosNivelesDeSeparadorParaSerInyectiva(t *testing.T) {
	// Con el mismo separador en los dos niveles, un conjunto de dos reglas y una
	// sola regla con dos campos podrían producir la misma cadena.
	maxAge, err := domStep.NewMaxAgeRule("24h")
	require.NoError(t, err)
	conjunto, err := domStep.NewRuleSet(domStep.NewDefaultStateChangedRule(), maxAge)
	require.NoError(t, err)

	canonical := conjunto.Canonical()
	assert.Equal(t, 1, strings.Count(canonical, "\x1f"), "un separador entre reglas")
	assert.Equal(t, 2, strings.Count(canonical, "\x1e"), "uno por regla, entre clave y parámetros")
}
