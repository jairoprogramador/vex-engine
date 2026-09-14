package pipeline_test

// El orden de ejecución de los steps es NUMÉRICO por el prefijo del directorio
// (spec 04 §5.2).
//
// Con dos dígitos el orden lexicográfico coincide con el numérico —`%02d` es de
// ancho fijo—, así que ordenar aquí no arregla un bug: enuncia el invariante en
// una línea legible en vez de dejarlo deducido de que `os.ReadDir` ordena por
// nombre. Los casos que la spec 00 escribió afirmando el orden roto de los
// prefijos de un dígito viven ahora en pipeline_structure_validator_test.go como
// errores de validación.

import (
	"testing"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	domPipeline "github.com/jairoprogramador/vex-engine/old-internal/domain/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func entradas(nombres ...string) []domPipeline.StepEntry {
	entries := make([]domPipeline.StepEntry, 0, len(nombres))
	for _, nombre := range nombres {
		entries = append(entries, domPipeline.StepEntry(nombre))
	}
	return entries
}

func ordenes(steps []command.StepName) []int {
	out := make([]int, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Order())
	}
	return out
}

func nombresCompletos(steps []command.StepName) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.FullName())
	}
	return out
}

func TestNewStepNames_OrdenNumerico(t *testing.T) {
	t.Run("12 steps de dos dígitos: el orden numérico se respeta", func(t *testing.T) {
		// Control de que la spec 04 no cambió lo que no debía: este caso ya pasaba
		// con la spec 00 y tiene que seguir pasando.
		steps, err := domPipeline.NewStepNames(entradas(
			"01-test", "02-supply", "03-package", "04-deploy", "05-notify",
			"06-audit", "07-scan", "08-sign", "09-publish", "10-promote",
			"11-verify", "12-cleanup",
		))

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, ordenes(steps))
		assert.Equal(t, "10-promote", steps[9].FullName(),
			"añadir el décimo step no reordena nada")
	})

	t.Run("el orden no lo hereda del orden de entrada", func(t *testing.T) {
		// La garantía deja de ser transitiva: aunque el repositorio devolviera los
		// directorios en cualquier orden, el dominio los ordena.
		steps, err := domPipeline.NewStepNames(entradas(
			"12-cleanup", "02-supply", "10-promote", "01-test"))

		require.NoError(t, err)
		assert.Equal(t,
			[]string{"01-test", "02-supply", "10-promote", "12-cleanup"},
			nombresCompletos(steps))
	})

	t.Run("sin entradas devuelve vacío", func(t *testing.T) {
		steps, err := domPipeline.NewStepNames(nil)

		require.NoError(t, err)
		assert.Empty(t, steps)
	})

	t.Run("una entrada inválida es un error que la nombra", func(t *testing.T) {
		// Segunda red: el validador ya la habría rechazado, pero el constructor no
		// puede producir un StepName cuyo FullName() no sea su directorio.
		_, err := domPipeline.NewStepNames(entradas("01-test", "2-supply"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "2-supply")
	})
}
