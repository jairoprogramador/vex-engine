package fingerprint_test

// Vectores y sensibilidades de la huella de variables (SPEC-VARIABLES-v1.md).

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

func variablesBase() []fingerprint.VariableMaterial {
	return []fingerprint.VariableMaterial{
		{Name: "region", Value: "us-east-1"},
		{Name: "replicas", Value: "3"},
		{Name: "bucket", Value: "artefactos", Shared: true},
	}
}

func huellaVars(t *testing.T, variables []fingerprint.VariableMaterial) string {
	t.Helper()
	huella, err := fingerprint.ComputeVariables(variables)
	require.NoError(t, err)
	return huella.String()
}

// --- §5 vectores ------------------------------------------------------------

func TestComputeVariables_Vectores(t *testing.T) {
	sinCompartida := variablesBase()
	sinCompartida[2].Shared = false

	conVacia := append(variablesBase(),
		fingerprint.VariableMaterial{Name: "instance_count", Value: ""})

	casos := []struct {
		numero    int
		variables []fingerprint.VariableMaterial
		huella    string
	}{
		{1, nil,
			"vars-v1:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{2, []fingerprint.VariableMaterial{{Name: "region", Value: "us-east-1"}},
			"vars-v1:7c7617e3218e094a48668b8d8ffa2d0993bb5c479a02a2c739b6e1a37213a5f0"},
		{3, variablesBase(),
			"vars-v1:a4941beb499496b2b29565fe8d0860acd7f9a286a4db2e5aa1e1661d58b1fced"},
		{4, sinCompartida,
			"vars-v1:dbadfd75e41583fa7cb333ca7f81809f6e2b2db30d517e4431a518711baa1ed8"},
		{5, conVacia,
			"vars-v1:22e49da2bbbb22b13b3b5b8c763dad14a310b9b9da761dc46c71e6de4fed4f41"},
	}

	for _, caso := range casos {
		t.Run(fmt.Sprintf("vector %d", caso.numero), func(t *testing.T) {
			assert.Equal(t, caso.huella, huellaVars(t, caso.variables),
				"vector %d de SPEC-VARIABLES-v1.md §5", caso.numero)
		})
	}
}

// --- §6.2 sensibilidades ----------------------------------------------------

func TestComputeVariables_Sensibilidades(t *testing.T) {
	base := huellaVars(t, variablesBase())

	t.Run("cambiar un valor cambia la huella", func(t *testing.T) {
		variables := variablesBase()
		variables[0].Value = "eu-west-1"
		assert.NotEqual(t, base, huellaVars(t, variables))
	})

	t.Run("cambiar sólo el ámbito compartido cambia la huella", func(t *testing.T) {
		variables := variablesBase()
		variables[0].Shared = true
		assert.NotEqual(t, base, huellaVars(t, variables))
	})

	t.Run("añadir una variable cambia la huella", func(t *testing.T) {
		variables := append(variablesBase(), fingerprint.VariableMaterial{Name: "extra", Value: "x"})
		assert.NotEqual(t, base, huellaVars(t, variables))
	})

	t.Run("declarada y vacía no es lo mismo que no declarada", func(t *testing.T) {
		// Spec 03 §5.3: el valor vacío es un dato legítimo. `Quote("")` es `""`,
		// no ausencia, así que la huella separa los dos estados.
		variables := append(variablesBase(), fingerprint.VariableMaterial{Name: "instance_count", Value: ""})
		assert.NotEqual(t, base, huellaVars(t, variables))
	})
}

// El orden de entrega no significa nada: un conjunto de variables no tiene
// secuencia (§3.3). Es la diferencia deliberada con la huella de instrucciones.
func TestComputeVariables_ElOrdenDeEntregaNoImporta(t *testing.T) {
	alReves := []fingerprint.VariableMaterial{
		{Name: "bucket", Value: "artefactos", Shared: true},
		{Name: "replicas", Value: "3"},
		{Name: "region", Value: "us-east-1"},
	}

	assert.Equal(t, huellaVars(t, variablesBase()), huellaVars(t, alReves))
}

// Los campos entrecomillados hacen la regla inyectiva (§3.2).
func TestComputeVariables_UnValorNoPuedeSimularElSeparador(t *testing.T) {
	conSeparador := []fingerprint.VariableMaterial{{Name: "a\x1eus-east-1", Value: ""}}
	partido := []fingerprint.VariableMaterial{{Name: "a", Value: "us-east-1"}}

	assert.NotEqual(t, huellaVars(t, conSeparador), huellaVars(t, partido))
}

func TestComputeVariables_UnValorNoPuedeSimularElSaltoDeLinea(t *testing.T) {
	unaSola := []fingerprint.VariableMaterial{{Name: "a\nb", Value: "v"}}
	dos := []fingerprint.VariableMaterial{{Name: "a", Value: "v"}, {Name: "b", Value: "v"}}

	assert.NotEqual(t, huellaVars(t, unaSola), huellaVars(t, dos))
}

func TestComputeVariables_LlevaSuPropioTokenDeVersion(t *testing.T) {
	huella, err := fingerprint.ComputeVariables(nil)
	require.NoError(t, err)

	assert.Equal(t, fingerprint.VariablesVersion, huella.Version())
	assert.NotEqual(t, fingerprint.Version, huella.Version())
	assert.NotEqual(t, fingerprint.InstructionsVersion, huella.Version())
	assert.Regexp(t, `^vars-v1:[0-9a-f]{64}$`, huella.String())
}
