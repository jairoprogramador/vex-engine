package pipeline_test

// Test de CARACTERIZACIÓN del orden de los steps (spec 00 §5.5).
//
// LA REGLA (spec 04): el directorio de un step es `NN-nombre` con EXACTAMENTE
// dos dígitos, y los steps se ejecutan en ORDEN NUMÉRICO por ese prefijo.
//
// Lo que hace el motor HOY:
//
//   - `PipelineStepRepository.Get` devuelve los steps en el orden de
//     `os.ReadDir`, que es lexicográfico por nombre. `StepName.Order()` se
//     parsea y nunca se usa para ordenar.
//   - `NewStepName` acepta `^(\d+)-(.+)$` — cualquier número de dígitos.
//
// Mientras todos los prefijos tengan dos dígitos, el orden lexicográfico
// COINCIDE con el numérico (`%02d` es de ancho fijo), así que el resultado es
// correcto por accidente. Deja de serlo en cuanto entra un prefijo de un
// dígito, y basta uno.
//
// Estos tests afirman el comportamiento ACTUAL. La spec 04 los cambia:
// el caso de dos dígitos debe seguir pasando; los de uno y de mezcla pasan a
// ser errores de validación y sus tests se borran allí.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	infraPipeline "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pipelineConSteps(t *testing.T, dirNames ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range dirNames {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "steps", name), 0o755))
	}
	return root
}

func getSteps(t *testing.T, pipelinePath string) []command.StepName {
	t.Helper()
	ctx := context.Background()
	steps, err := infraPipeline.NewPipelineStepRepository().Get(&ctx, pipelinePath)
	require.NoError(t, err)
	return steps
}

func fullNames(steps []command.StepName) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.FullName())
	}
	return out
}

func orders(steps []command.StepName) []int {
	out := make([]int, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.Order())
	}
	return out
}

func TestPipelineStepRepository_Get_OrdenDeLosSteps(t *testing.T) {
	t.Run("12 steps de dos dígitos: el orden numérico se respeta", func(t *testing.T) {
		// La regla se cumple hoy, y debe seguir cumpliéndose tras la spec 04.
		// Este caso NO se borra allí: es el control de que la spec 04 no
		// cambió lo que no debía.
		path := pipelineConSteps(t,
			"01-test", "02-supply", "03-package", "04-deploy", "05-notify",
			"06-audit", "07-scan", "08-sign", "09-publish", "10-promote",
			"11-verify", "12-cleanup",
		)

		steps := getSteps(t, path)

		assert.Equal(t, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, orders(steps),
			"con ancho fijo el orden lexicográfico ES el numérico, sin límite de steps")
		assert.Equal(t, "10-promote", steps[9].FullName(),
			"añadir el décimo step no reordena nada")
	})

	t.Run("prefijo de UN dígito: el orden numérico NO se respeta", func(t *testing.T) {
		// DEFECTO VIVO: `NewStepName` acepta `\d+`, así que estos directorios
		// se cargan, y `os.ReadDir` los ordena por nombre.
		// SPEC 04: pasa a ser error de validación; este caso se borra allí.
		path := pipelineConSteps(t,
			"1-test", "2-supply", "3-package", "4-deploy", "5-notify",
			"6-audit", "7-scan", "8-sign", "9-publish", "10-promote",
			"11-verify", "12-cleanup",
		)

		steps := getSteps(t, path)

		assert.Equal(t, []int{1, 10, 11, 12, 2, 3, 4, 5, 6, 7, 8, 9}, orders(steps),
			"10-promote se ejecuta ANTES que 2-supply")
		assert.Equal(t, "10-promote", steps[1].FullName(),
			"segundo step ejecutado: el que el autor puso décimo")
	})

	t.Run("basta UN directorio mal nombrado para reordenar el pipeline", func(t *testing.T) {
		// SPEC 04: pasa a ser error de validación; este caso se borra allí.
		path := pipelineConSteps(t, "01-test", "2-supply", "03-package")

		steps := getSteps(t, path)

		assert.Equal(t, []int{1, 3, 2}, orders(steps),
			"el step 2 se ejecuta el último")
	})

	t.Run("FullName reformatea a dos dígitos y colapsa dos nombres en uno", func(t *testing.T) {
		// DEFECTO VIVO: `2-supply` y `02-supply` producen la MISMA clave
		// canónica, así que dos pipelinecodes distintos comparten estado
		// persistido. Además el motor buscará `steps/02-supply/commands.yaml`,
		// que no existe, y el step pasará sin ejecutar nada.
		// SPEC 04: `NewStepName("2-supply")` pasa a devolver error y `%02d`
		// deja de normalizar nada; este caso se borra allí.
		path := pipelineConSteps(t, "2-supply")

		steps := getSteps(t, path)

		require.Len(t, steps, 1)
		assert.Equal(t, "02-supply", steps[0].FullName(),
			"la misma clave que produciría un directorio 02-supply")
		assert.Equal(t, "supply", steps[0].Name())
	})
}

func TestPipelineStepRepository_Get_QuePrefijosSeAceptan(t *testing.T) {
	// La regla (spec 04) es EXACTAMENTE dos dígitos. Hoy el regex es `\d+`, así
	// que acepta uno, tres o los que sean. Esta tabla pinnea qué entra hoy;
	// la spec 04 convierte en error todo lo que no sean dos dígitos.
	cases := []struct {
		dirName  string
		aceptado bool
		fullName string
		nota     string
	}{
		{dirName: "02-supply", aceptado: true, fullName: "02-supply", nota: "la regla"},
		{
			dirName: "2-supply", aceptado: true, fullName: "02-supply",
			nota: "DEFECTO VIVO: un dígito se acepta y se normaliza",
		},
		{
			dirName: "002-supply", aceptado: true, fullName: "02-supply",
			nota: "DEFECTO VIVO: tres dígitos también, y colapsan en la misma clave",
		},
		{
			dirName: "123-supply", aceptado: true, fullName: "123-supply",
			nota: "un orden de tres cifras se conserva: %02d no trunca",
		},
		{dirName: "supply", aceptado: false, nota: "sin prefijo numérico"},
		{dirName: "package-02", aceptado: false, nota: "el número va delante"},
		{dirName: "-deploy", aceptado: false, nota: "prefijo vacío"},
		{dirName: "04_test", aceptado: false, nota: "separador guion bajo"},
		{dirName: "02-", aceptado: false, nota: "nombre vacío"},
	}

	for _, tc := range cases {
		t.Run(tc.dirName, func(t *testing.T) {
			steps := getSteps(t, pipelineConSteps(t, tc.dirName))

			if !tc.aceptado {
				assert.Empty(t, steps, "se descarta EN SILENCIO. %s", tc.nota)
				return
			}
			require.Len(t, steps, 1, tc.nota)
			assert.Equal(t, tc.fullName, steps[0].FullName(), tc.nota)
		})
	}
}

func TestPipelineStepRepository_Get_QueSeIgnora(t *testing.T) {
	t.Run("los directorios que no siguen NN-nombre se descartan en silencio", func(t *testing.T) {
		path := pipelineConSteps(t, "01-test", "supply", "package-02", "-deploy", "02-audit")

		steps := getSteps(t, path)

		assert.Equal(t, []string{"01-test", "02-audit"}, fullNames(steps))
	})

	t.Run("los archivos sueltos se ignoran", func(t *testing.T) {
		path := pipelineConSteps(t, "01-test")
		require.NoError(t, os.WriteFile(
			filepath.Join(path, "steps", "02-noesundirectorio"), []byte("x"), 0o644))

		steps := getSteps(t, path)

		assert.Equal(t, []string{"01-test"}, fullNames(steps))
	})

	t.Run("sin directorio steps devuelve vacío SIN error", func(t *testing.T) {
		steps := getSteps(t, t.TempDir())

		assert.Empty(t, steps)
		assert.NotNil(t, steps, "devuelve slice vacío, no nil")
	})

	t.Run("directorio steps vacío devuelve vacío", func(t *testing.T) {
		path := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(path, "steps"), 0o755))

		assert.Empty(t, getSteps(t, path))
	})
}
