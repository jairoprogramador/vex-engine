package step_test

// El material que la cadena de pipeline carga y la de step consume (spec 18
// §5.2).
//
// Sólo hay una decisión que probar aquí, y es la que separa un defecto del motor
// de un desenlace legítimo: pedir el material de un step que nadie cargó.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
)

// La ausencia es un ERROR y no material vacío. El conjunto vacío de comandos
// significa otra cosa —un step sin comandos se salta con `no_commands`
// (spec 04 §5.3)— así que devolverlo aquí convertiría un defecto del motor en un
// step silenciosamente saltado.
func TestLoadedPipelinecode_PedirLoQueNadieCargoEsUnError(t *testing.T) {
	cargado := domStep.NewLoadedPipelinecode()
	cargado.Put("01-test", domStep.LoadedStep{})

	_, err := cargado.Get("02-supply")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "02-supply")
	assert.Contains(t, err.Error(), "01-test", "el error dice contra qué se comparó")
}

// La clave es el nombre del DIRECTORIO, con su prefijo de orden: la misma
// identidad que usan `state.Key` y `deployment.StepContent`.
func TestLoadedPipelinecode_ElMaterialViajaEntero(t *testing.T) {
	cmd, err := command.NewCommand("provision", "echo hola")
	require.NoError(t, err)

	declaration, err := domStep.NewLiteralDeclaration("registry_prefix", "vexsand")
	require.NoError(t, err)

	config, err := domStep.NewStepConfig(domStep.NewProjectScope(), domStep.EmptyRuleSet())
	require.NoError(t, err)

	cargado := domStep.NewLoadedPipelinecode()
	cargado.Put("02-supply", domStep.LoadedStep{
		Commands:     []command.Command{cmd},
		Config:       config,
		Declarations: []domStep.VariableDeclaration{declaration},
	})

	material, err := cargado.Get("02-supply")
	require.NoError(t, err)

	assert.Len(t, material.CommandsCopy(), 1)
	assert.Len(t, material.DeclarationsCopy(), 1)
	assert.True(t, material.Config.Scope().IsProject())
	assert.Equal(t, []string{"02-supply"}, cargado.StepIDs())
}
