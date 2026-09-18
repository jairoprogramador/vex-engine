package dominio_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/resolucion/dominio"
)

func TestInterpolarSustituyeCadaVariablePorSuValor(t *testing.T) {
	disponibles := []dominio.VariableEfectiva{
		declarada(t, "host", "vexja.com", ambienteProd(t)),
		declarada(t, "puerto", "443", ambienteProd(t)),
	}
	resultado, err := dominio.Interpolar("https://${var.host}:${var.puerto}/", disponibles)
	require.NoError(t, err)
	require.Equal(t, "https://vexja.com:443/", resultado)
}

func TestInterpolarSinVariablesDevuelveElTextoIgual(t *testing.T) {
	resultado, err := dominio.Interpolar("sin nada que sustituir", nil)
	require.NoError(t, err)
	require.Equal(t, "sin nada que sustituir", resultado)
}

func TestInterpolarFallaConUnNombreQueNoEsta(t *testing.T) {
	_, err := dominio.Interpolar("${var.no_existe}", nil)
	var noEncontrada *dominio.VariableNoEncontradaError
	require.True(t, errors.As(err, &noEncontrada))
	require.Equal(t, "no_existe", noEncontrada.Nombre)
	require.ErrorIs(t, err, dominio.ErrRechazado)
}
