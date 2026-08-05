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
		_, err := command.NewVariable("", "us-east-1", false)
		require.ErrorIs(t, err, command.ErrVariableNameEmpty)
	})

	t.Run("el valor vacío ya NO es un error", func(t *testing.T) {
		// Cierra D-A8: habilita el parámetro opcional que las specs del
		// registro necesitan (`secrets_required`, `vex plan`).
		variable, err := command.NewVariable("instance_count", "", false)
		require.NoError(t, err)

		assert.Equal(t, "instance_count", variable.Name())
		assert.Equal(t, "", variable.Value())
		assert.False(t, variable.IsShared())
	})

	t.Run("el nombre y el ámbito se conservan", func(t *testing.T) {
		variable, err := command.NewVariable("bucket", "artefactos", true)
		require.NoError(t, err)

		assert.Equal(t, "bucket", variable.Name())
		assert.Equal(t, "artefactos", variable.Value())
		assert.True(t, variable.IsShared())
	})
}

// Una variable declarada y vacía es indistinguible de una no declarada si se
// mira solo el mapa, pero no lo es en el mapa: la clave existe. Es la
// precondición de que la huella las distinga (ver
// TestVariablesRule_MaterialDeLaHuella).
func TestNewVariable_DeclaradaYVaciaOcupaSuClave(t *testing.T) {
	vars := command.NewExecutionVariableMap()

	variable, err := command.NewVariable("instance_count", "", false)
	require.NoError(t, err)
	vars.Add(variable)

	valor, existe := vars.Get("instance_count")
	require.True(t, existe)
	assert.Equal(t, "", valor.Value())

	_, existeAnonima := vars.Get("")
	assert.False(t, existeAnonima, "ninguna variable puede ocupar la clave vacía")
}
