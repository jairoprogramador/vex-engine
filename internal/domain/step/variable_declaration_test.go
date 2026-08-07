package step_test

// La gramática de la spec 14 §5.2 y lo que cada origen exige.
//
// Lo que se fija aquí es que el vocabulario sea CERRADO y que los campos
// obligatorios los pida el constructor: `from` y `key` ausentes en un
// `step-output` son un ERROR, no un default. Un default sería adivinar cuál es el
// step productor, que es exactamente la alternativa D —inferir del `outputs` de
// los demás— y quedaría congelado dentro de un identificador permanente.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

func TestVariableSource_ElVocabularioEsCerrado(t *testing.T) {
	t.Run("la ausencia de resolve es el literal de siempre", func(t *testing.T) {
		source, err := domStep.NewVariableSource("")
		require.NoError(t, err)
		assert.Equal(t, domStep.SourceLiteral, source)
		assert.Equal(t, domStep.VariableSource(""), domStep.SourceLiteral,
			"el literal es el valor CERO: un pipelinecode de siempre significa lo mismo sin tocar una línea")
	})

	for _, token := range []string{"step-output", "state"} {
		t.Run(token, func(t *testing.T) {
			source, err := domStep.NewVariableSource(token)
			require.NoError(t, err)
			assert.Equal(t, token, string(source))
		})
	}

	for _, inventado := range []string{"dynamic", "Step-Output", "output", "env"} {
		t.Run("rechaza '"+inventado+"'", func(t *testing.T) {
			_, err := domStep.NewVariableSource(inventado)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "step-output")
			assert.Contains(t, err.Error(), "state")
		})
	}
}

func TestVariableDeclaration_CadaOrigenExigeSusCampos(t *testing.T) {
	casos := []struct {
		nombre    string
		construir func() (domStep.VariableDeclaration, error)
		valido    bool
		enElError string
	}{
		{
			nombre:    "literal con valor",
			construir: func() (domStep.VariableDeclaration, error) { return domStep.NewLiteralDeclaration("n", "3") },
			valido:    true,
		},
		{
			nombre:    "literal con valor VACÍO",
			construir: func() (domStep.VariableDeclaration, error) { return domStep.NewLiteralDeclaration("n", "") },
			valido:    true,
			// Herencia de la spec 03 §5.1: «declarada y vacía» y «no declarada» son
			// estados distintos. La asimetría con `outputs` —donde un grupo de
			// captura vacío SÍ es un fallo— la conserva `CommandVariable`.
		},
		{
			nombre:    "sin name",
			construir: func() (domStep.VariableDeclaration, error) { return domStep.NewLiteralDeclaration("", "3") },
			valido:    false,
			enElError: "name",
		},
		{
			nombre: "step-output completo",
			construir: func() (domStep.VariableDeclaration, error) {
				return domStep.NewStepOutputDeclaration("acr", "02-supply", "acr_name")
			},
			valido: true,
		},
		{
			nombre: "step-output sin from",
			construir: func() (domStep.VariableDeclaration, error) {
				return domStep.NewStepOutputDeclaration("acr", "", "acr_name")
			},
			valido:    false,
			enElError: "'from'",
		},
		{
			nombre: "step-output sin key",
			construir: func() (domStep.VariableDeclaration, error) {
				return domStep.NewStepOutputDeclaration("acr", "02-supply", "")
			},
			valido:    false,
			enElError: "'key'",
		},
		{
			nombre: "state completo",
			construir: func() (domStep.VariableDeclaration, error) {
				return domStep.NewStateDeclaration("DB_HOST", "project", "lb-arn")
			},
			valido: true,
		},
		{
			nombre: "state sin scope",
			construir: func() (domStep.VariableDeclaration, error) {
				return domStep.NewStateDeclaration("DB_HOST", "", "lb-arn")
			},
			valido:    false,
			enElError: "'scope'",
		},
		{
			nombre: "state con un ámbito inventado",
			construir: func() (domStep.VariableDeclaration, error) {
				return domStep.NewStateDeclaration("DB_HOST", "global", "lb-arn")
			},
			valido:    false,
			enElError: "global",
		},
		{
			nombre: "state sin key",
			construir: func() (domStep.VariableDeclaration, error) {
				return domStep.NewStateDeclaration("DB_HOST", "project", "")
			},
			valido:    false,
			enElError: "'key'",
		},
	}

	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			_, err := caso.construir()
			if caso.valido {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), caso.enElError)
		})
	}
}

// La forma canónica es el material de identidad, y su propiedad es DISCRIMINAR.
//
// Es el argumento cerrado contra el marcador genérico `dynamic: true`: dos
// configuraciones distintas tienen que producir cadenas distintas, y un «esto se
// resuelve luego» produciría la misma para todas.
func TestVariableDeclaration_LaFormaCanonicaDiscrimina(t *testing.T) {
	canonicas := make(map[string]string)

	declaraciones := map[string]func() (domStep.VariableDeclaration, error){
		"literal '3'":   func() (domStep.VariableDeclaration, error) { return domStep.NewLiteralDeclaration("n", "3") },
		"literal '4'":   func() (domStep.VariableDeclaration, error) { return domStep.NewLiteralDeclaration("n", "4") },
		"literal vacío": func() (domStep.VariableDeclaration, error) { return domStep.NewLiteralDeclaration("n", "") },
		"output de 01-test": func() (domStep.VariableDeclaration, error) {
			return domStep.NewStepOutputDeclaration("n", "01-test", "k")
		},
		"output de 02-supply": func() (domStep.VariableDeclaration, error) {
			return domStep.NewStepOutputDeclaration("n", "02-supply", "k")
		},
		"otro key del mismo": func() (domStep.VariableDeclaration, error) {
			return domStep.NewStepOutputDeclaration("n", "01-test", "otra")
		},
		"state de project": func() (domStep.VariableDeclaration, error) { return domStep.NewStateDeclaration("n", "project", "k") },
		"state de environment": func() (domStep.VariableDeclaration, error) {
			return domStep.NewStateDeclaration("n", "environment", "k")
		},
		"state con otro key": func() (domStep.VariableDeclaration, error) {
			return domStep.NewStateDeclaration("n", "project", "otra")
		},
	}

	for nombre, construir := range declaraciones {
		declaration, err := construir()
		require.NoError(t, err)

		canonical := declaration.Canonical()
		if anterior, repetida := canonicas[canonical]; repetida {
			t.Fatalf("'%s' y '%s' producen la misma forma canónica %q", nombre, anterior, canonical)
		}
		canonicas[canonical] = nombre
	}

	t.Run("y es estable: la misma declaración da la misma cadena", func(t *testing.T) {
		primera, err := domStep.NewStepOutputDeclaration("acr", "02-supply", "acr_name")
		require.NoError(t, err)
		segunda, err := domStep.NewStepOutputDeclaration("acr", "02-supply", "acr_name")
		require.NoError(t, err)

		assert.Equal(t, primera.Canonical(), segunda.Canonical())
	})
}

// El mensaje nombra LA FUENTE. Es la diferencia observable de §5.5: hasta esta
// spec una variable que no llegaba fallaba con «variable no existe», que no dice
// a quién reclamarle.
func TestVariableDeclaration_ElMensajeNombraLaFuente(t *testing.T) {
	deOutput, err := domStep.NewStepOutputDeclaration("acr", "02-supply", "acr_name")
	require.NoError(t, err)
	assert.Contains(t, deOutput.SourceDescription(), "02-supply")
	assert.Contains(t, deOutput.SourceDescription(), "acr_name")

	deEstado, err := domStep.NewStateDeclaration("DB_HOST", "project", "lb-arn")
	require.NoError(t, err)
	assert.Contains(t, deEstado.SourceDescription(), "project")
	assert.Contains(t, deEstado.SourceDescription(), "lb-arn")
}
