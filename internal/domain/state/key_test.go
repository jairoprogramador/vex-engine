package state_test

// La clave de POSICIÓN: qué entra y qué no (spec 11 §5.1 y §5.2).
//
// Los dos casos que fijan la tesis de la spec son «el ámbito separa» y «el
// pipeline no está»; el resto son los invariantes que impiden una clave con un
// hueco.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

const proyecto = "https://vex.test/acme/demo-app.git"

func TestKey_ElAmbitoSepara(t *testing.T) {
	prod, err := state.NewEnvironmentScope("prod")
	require.NoError(t, err)
	sand, err := state.NewEnvironmentScope("sand")
	require.NoError(t, err)

	enProd, err := state.NewKey(proyecto, prod, "02-supply")
	require.NoError(t, err)
	enSand, err := state.NewKey(proyecto, sand, "02-supply")
	require.NoError(t, err)
	enProyecto, err := state.NewKey(proyecto, state.NewProjectScope(), "02-supply")
	require.NoError(t, err)

	// El aislamiento entre ambientes que la spec 10 compró metiendo el ambiente
	// en el hash: la misma garantía, y ya no puede caerse de ningún hash porque
	// no está en ninguno.
	assert.False(t, enProd.Equals(enSand))
	assert.False(t, enProd.Equals(enProyecto))
	assert.True(t, enProd.Equals(mustKey(t, proyecto, prod, "02-supply")))
}

// D-A14 cerrada al revés: el ACR pertenece al proyecto, no al pipeline. Dos
// pipelines sobre el mismo proyecto comparten clave A PROPÓSITO — lo que impide
// que uno reviva el registro del otro es la huella, no la clave (ver
// TestStepRecord_DosPipelinesNoSeRevivenEntreSi).
func TestKey_NoLlevaElPipeline(t *testing.T) {
	prod, err := state.NewEnvironmentScope("prod")
	require.NoError(t, err)

	key, err := state.NewKey(proyecto, prod, "02-supply")
	require.NoError(t, err)

	assert.Equal(t, proyecto, key.Subject())
	assert.Equal(t, "environment:prod", key.Scope().String())
	assert.Equal(t, "02-supply", key.StepID())
	assert.NotContains(t, key.String(), "pipeline")
}

// El step_id lleva el prefijo de orden, así que renumerar pierde la historia.
// Es un coste declarado (spec 11 §5.3) y el test existe para que sea visible:
// el efecto es una re-ejecución, nunca un despliegue omitido.
func TestKey_RenumerarUnStepEsOtroStep(t *testing.T) {
	scope := state.NewProjectScope()

	antes := mustKey(t, proyecto, scope, "02-supply")
	despues := mustKey(t, proyecto, scope, "03-supply")

	assert.False(t, antes.Equals(despues))
}

func TestKey_UnComponenteAusenteEsUnError(t *testing.T) {
	prod, err := state.NewEnvironmentScope("prod")
	require.NoError(t, err)

	casos := []struct {
		nombre  string
		subject string
		scope   state.Scope
		stepID  string
	}{
		{"sin subject", "", prod, "02-supply"},
		{"sin ámbito", proyecto, state.Scope{}, "02-supply"},
		{"sin step", proyecto, prod, ""},
		{"un step que sale de su directorio", proyecto, prod, "../otro"},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			_, err := state.NewKey(caso.subject, caso.scope, caso.stepID)
			assert.Error(t, err, "una clave con un hueco lee el estado de otro step")
		})
	}
}

func TestScope_FormaLogicaYFormaEnRuta(t *testing.T) {
	prod, err := state.NewEnvironmentScope("prod")
	require.NoError(t, err)

	// La forma lógica lleva los dos puntos; la ruta NO, porque `:` es ilegal en
	// rutas de Windows y vexd se compila también para Windows (spec 11 §5.4).
	assert.Equal(t, "environment:prod", prod.String())
	assert.Equal(t, []string{"environment", "prod"}, prod.Segments())

	proyecto := state.NewProjectScope()
	assert.Equal(t, "project", proyecto.String())
	assert.Equal(t, []string{"project"}, proyecto.Segments())
}

// Un ambiente llamado `project` da `environment:project` y no colisiona con
// nada: el ámbito de ambiente viaja SIEMPRE prefijado. Es lo que permite que la
// spec 13 retire la palabra reservada `shared`.
func TestScope_ElAmbienteViajaPrefijado(t *testing.T) {
	homonimo, err := state.NewEnvironmentScope("project")
	require.NoError(t, err)

	assert.Equal(t, "environment:project", homonimo.String())
	assert.False(t, homonimo.Equals(state.NewProjectScope()))
}

func TestScope_RoundTrip(t *testing.T) {
	casos := []string{"project", "environment:prod", "environment:sand"}

	for _, texto := range casos {
		t.Run(texto, func(t *testing.T) {
			scope, err := state.ParseScope(texto)
			require.NoError(t, err)
			assert.Equal(t, texto, scope.String())
		})
	}

	for _, texto := range []string{"", "shared", "environment:", "environment", "otro:cosa"} {
		t.Run("rechaza "+texto, func(t *testing.T) {
			_, err := state.ParseScope(texto)
			assert.Error(t, err)
		})
	}
}

// Un nombre de ambiente que rompe la ruta no debe producir una ruta rara: debe
// producir un error, porque el ámbito ES un tramo de la ruta del almacén.
func TestScope_UnNombreQueRompeLaRutaSeRechaza(t *testing.T) {
	for _, nombre := range []string{"", "..", ".", "pro/duccion", `pro\duccion`, "env:prod"} {
		t.Run(nombre, func(t *testing.T) {
			_, err := state.NewEnvironmentScope(nombre)
			assert.Error(t, err)
		})
	}
}

func mustKey(t *testing.T, subject string, scope state.Scope, stepID string) state.Key {
	t.Helper()
	key, err := state.NewKey(subject, scope, stepID)
	require.NoError(t, err)
	return key
}
