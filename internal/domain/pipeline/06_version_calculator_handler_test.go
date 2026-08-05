package pipeline_test

// Dos relojes en la misma función (spec 07, hallazgo f).
//
// `06_version_calculator_handler` tomaba el instante de `time.Now()` para la
// rama de semver y del `startedAt` de la ejecución para la rama de fecha: la
// MISMA decisión —qué fecha lleva la versión— se tomaba con dos fuentes según
// por dónde entrara. Con el reloj inyectado hay una sola, y por eso esto se
// puede afirmar en una prueba: antes el instante estaba dentro del handler y
// no era observable.

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

var instanteDelReloj = time.Date(2026, 3, 4, 10, 30, 0, 0, time.UTC)

// repositorioDeTags devuelve un árbol sin cambios versionables: hay un tag
// previo pero ningún commit convencional, que es la rama por la que la versión
// se calcula por FECHA. Es donde el reloj se ve.
type repositorioDeTags struct{ ultimoTag string }

func (r repositorioDeTags) LastTag(*context.Context, string) (string, error) {
	return r.ultimoTag, nil
}

func (r repositorioDeTags) RecentCommits(*context.Context, string, string, int) (string, []string, error) {
	return "abc123def456789", []string{"chore: nada que versionar"}, nil
}

type emisorMudo struct{}

func (emisorMudo) Notify(string, string) {}
func (emisorMudo) Close()                {}

func versionCalculadaPara(t *testing.T, step string) string {
	t.Helper()

	ctx := context.Background()
	ejecucion := command.NewExecution(
		command.NewExecutionID("exec-1"),
		command.NewExecutionProject("id", "proyecto", "https://vex.test/org/proyecto.git", "main", "org", "equipo"),
		command.NewExecutionPipeline("https://vex.test/org/pipeline.git", "main"),
		step,
		"prod",
		command.NewExecutionRuntime("", ""),
		// El agregado arranca con un instante DISTINTO al del handler: si alguna
		// rama volviera a leer el `startedAt` de la ejecución en vez del reloj,
		// las dos versiones dejarían de coincidir y este test lo diría.
		shared.NewFixedClock(instanteDelReloj.Add(-72*time.Hour)),
	)
	executionContext := command.NewExecutionContext(&ctx, ejecucion, nil, nil, emisorMudo{}, nil)

	handler := domPipeline.NewVersionCalculatorHandler(
		repositorioDeTags{ultimoTag: "v1.2.3"},
		shared.NewFixedClock(instanteDelReloj),
	)
	request := domPipeline.NewPipelineRequestHandler(executionContext)
	require.NoError(t, handler.Handle(&ctx, request))

	assert.Equal(t, "abc123def456789", request.ProjectHeadHash())
	return request.ProjectVersion()
}

func TestVersionCalculator_LasDosRamasUsanLaMismaFuenteDeTiempo(t *testing.T) {
	esperada := domPipeline.NewDateVersion(instanteDelReloj).String()

	// `deploy` entra por la rama de semver, que cae en versión por fecha cuando
	// no hay commits versionables; cualquier otro step entra por la rama de
	// fecha directamente. Antes cada una miraba un reloj distinto.
	assert.Equal(t, esperada, versionCalculadaPara(t, "deploy"))
	assert.Equal(t, esperada, versionCalculadaPara(t, "supply"))
}

func TestVersionCalculator_ConRelojFijoDosEjecucionesDanLaMismaVersion(t *testing.T) {
	assert.Equal(t, versionCalculadaPara(t, "supply"), versionCalculadaPara(t, "supply"),
		"con el instante dentro del handler esto no era verificable")
}
