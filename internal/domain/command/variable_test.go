package command_test

// El invariante de Variable, partido en dos (spec 03 §5.1).
//
// Antes, un solo `if` defendía nombre y valor. Son invariantes de naturaleza
// distinta: el nombre siempre lo pone el motor o el `name:` del pipelinecode
// —un vacío ahí es un bug—, mientras que el valor viene de datos del usuario y
// un vacío ahí es un dato.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

func TestNewVariable_Invariante(t *testing.T) {
	t.Run("el nombre vacío sigue siendo un error", func(t *testing.T) {
		_, err := command.NewVariable("", "us-east-1", command.OriginDeclared)
		require.ErrorIs(t, err, command.ErrVariableNameEmpty)
	})

	t.Run("el valor vacío ya NO es un error", func(t *testing.T) {
		// Cierra D-A8: habilita el parámetro opcional que las specs del
		// registro necesitan (`secrets_required`, `vex plan`).
		variable, err := command.NewVariable("instance_count", "", command.OriginDeclared)
		require.NoError(t, err)

		assert.Equal(t, "instance_count", variable.Name())
		assert.Equal(t, "", variable.Value())
	})

	t.Run("el nombre, el valor y el origen se conservan", func(t *testing.T) {
		// El ÁMBITO ya no está en esta lista, y su ausencia es el entregable de la
		// spec 13 §5.6: era un atributo por variable cuando es del STEP. Aquí se
		// afirmaba `IsShared()`, que ya no existe.
		variable, err := command.NewVariable("bucket", "artefactos", command.OriginRuntime)
		require.NoError(t, err)

		assert.Equal(t, "bucket", variable.Name())
		assert.Equal(t, "artefactos", variable.Value())
		assert.Equal(t, command.OriginRuntime, variable.Origin())
	})
}

// El orden del `iota` ES la precedencia (spec 12 §5.1), así que es una decisión
// del modelo y no un detalle de implementación: reordenar las constantes cambia
// qué valor gana en cada ejecución del motor.
//
// El valor cero es `OriginDeclared`, la precedencia más baja, a propósito: un
// llamador que olvide declarar el origen produce una variable que no pisa a
// nadie en vez de una que lo pisa todo.
func TestOrigin_ElOrdenEsLaPrecedencia(t *testing.T) {
	assert.Equal(t, command.OriginDeclared, command.Origin(0),
		"el valor cero es el default, no la autoridad")

	assert.Less(t, command.OriginDeclared, command.OriginState,
		"un literal es un valor por defecto: lo almacenado gana (P3)")
	assert.Less(t, command.OriginState, command.OriginInjected,
		"un hecho de ESTA ejecución no lo pisa uno de una corrida anterior")
	assert.Less(t, command.OriginInjected, command.OriginRuntime,
		"lo producido al ejecutar gana sobre todo")

	assert.Equal(t, "declared", command.OriginDeclared.String())
	assert.Equal(t, "state", command.OriginState.String())
	assert.Equal(t, "injected", command.OriginInjected.String())
	assert.Equal(t, "runtime", command.OriginRuntime.String())
}

// La lista de variables volátiles es NORMATIVA: está transcrita en
// `fingerprint/SPEC-VARIABLES-v1.md` §3.1, y de ella depende que un paso pueda
// saltarse alguna vez —las seis cambian solas— y que dos máquinas obtengan la
// misma `cache_key` —tres de ellas son rutas absolutas—.
//
// Hasta la spec 10 la misma lista estaba escrita dos veces, en `vars_rule.go` y
// en `step_executable.go`, sin nada que las mantuviera sincronizadas. Este test
// es lo que sustituye a esa coincidencia: si alguien añade o quita un nombre,
// tiene que venir aquí y a la especificación.
func TestVolatileVarNames_EsLaListaDeLaEspecificacion(t *testing.T) {
	assert.Equal(t, []string{
		"project_version",
		"project_revision",
		"project_revision_full",
		"tool_name",
		"project_workdir",
		"step_workdir",
	}, command.VolatileVarNames())

	assert.True(t, command.IsVolatileVar(command.VarStepWorkdir))
	assert.False(t, command.IsVolatileVar(command.VarEnvironment),
		"`environment` NO es volátil: entra en el material de la huella de variables")
}

// Una variable declarada y vacía es indistinguible de una no declarada si se
// mira solo el mapa, pero no lo es en el mapa: la clave existe. Es la
// precondición de que la huella las distinga (ver
// TestComputeVariables_Sensibilidades).
func TestNewVariable_DeclaradaYVaciaOcupaSuClave(t *testing.T) {
	vars := command.NewExecutionVariableMap()

	variable, err := command.NewVariable("instance_count", "", command.OriginDeclared)
	require.NoError(t, err)
	vars.Add(variable)

	valor, existe := vars.Get("instance_count")
	require.True(t, existe)
	assert.Equal(t, "", valor.Value())

	_, existeAnonima := vars.Get("")
	assert.False(t, existeAnonima, "ninguna variable puede ocupar la clave vacía")
}
