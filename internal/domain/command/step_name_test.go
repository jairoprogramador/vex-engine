package command_test

// El constructor exige EXACTAMENTE dos dígitos (spec 04 §5.1).
//
// No es redundante con el validador de estructura, y la diferencia importa: el
// validador existe para dar un mensaje accionable sobre un directorio que no se
// pudo convertir en step; el constructor existe para que NO EXISTA un StepName
// cuyo `FullName()` no coincida con el directorio del que salió.
//
// Hasta la spec 04 sí existía: `NewStepName("2-supply").FullName()` era
// "02-supply", y de ahí salían los dos síntomas del defecto (b′) —el
// `commands.yaml` que el motor no encuentra, y la clave de estado compartida
// entre dos pipelinecodes distintos—. El test de la spec 00 que afirmaba esa
// normalización se borró aquí.

import (
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStepName_QuePrefijosSeAceptan(t *testing.T) {
	casos := []struct {
		dirName  string
		aceptado bool
		nota     string
	}{
		{dirName: "01-test", aceptado: true, nota: "la regla"},
		{dirName: "02-supply", aceptado: true},
		{dirName: "12-cleanup", aceptado: true, nota: "pasar de nueve steps es legítimo"},
		{dirName: "00-preflight", aceptado: true, nota: "cero es un orden válido"},
		{dirName: "02-supply-remoto", aceptado: true, nota: "el nombre puede llevar guiones"},
		{dirName: "2-supply", nota: "un dígito: hacía desaparecer el step"},
		{dirName: "002-supply", nota: "tres dígitos: colapsaba en la misma clave que 02-supply"},
		{dirName: "123-supply", nota: "tres dígitos, aunque %02d no truncara"},
		{dirName: "supply", nota: "sin prefijo numérico"},
		{dirName: "package-02", nota: "el número va delante"},
		{dirName: "-deploy", nota: "prefijo vacío"},
		{dirName: "04_test", nota: "separador guion bajo"},
		{dirName: "02-", nota: "nombre vacío"},
		{dirName: "", nota: "cadena vacía"},
	}

	for _, caso := range casos {
		t.Run(caso.dirName, func(t *testing.T) {
			stepName, err := command.NewStepName(caso.dirName)

			if !caso.aceptado {
				require.Error(t, err, caso.nota)
				assert.Contains(t, err.Error(), caso.dirName, "el error nombra el directorio")
				return
			}
			require.NoError(t, err, caso.nota)
			assert.Equal(t, caso.dirName, stepName.FullName(),
				"INVARIANTE: FullName() es el directorio del que salió, no una normalización suya")
		})
	}
}

func TestStepName_FullNameEsLaIdentidad(t *testing.T) {
	// Con el prefijo validado, `%02d` reformatea a lo que ya era. Se conserva
	// porque es el formato canónico de la clave de estado, no porque tolere nada
	// (spec 04 §8).
	for _, dirName := range []string{"00-a", "01-test", "09-publish", "10-promote", "99-z"} {
		stepName, err := command.NewStepName(dirName)
		require.NoError(t, err)
		assert.Equal(t, dirName, stepName.FullName())
	}
}

func TestStepName_OrderYName(t *testing.T) {
	stepName, err := command.NewStepName("02-supply")

	require.NoError(t, err)
	assert.Equal(t, 2, stepName.Order())
	assert.Equal(t, "supply", stepName.Name())
}
