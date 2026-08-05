package pipeline_test

// Los tres templates reales pasan la validación de la spec 04 SIN modificarse
// (spec 04 §6: «conviene verificarlo con un test, no asumirlo»).
//
// Los templates viven fuera de este repo, en `Vex/pipelines/`, así que el test se
// salta cuando no están al lado. Eso es deliberado: en el workspace comprueba los
// archivos de verdad, y en un checkout aislado de vex-engine no inventa un fallo
// sobre algo que no puede ver.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	infraPipeline "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// templatesPath es la ruta a `Vex/pipelines/` desde este paquete.
const templatesPath = "../../../../../pipelines"

var plantillasReales = []string{
	"vex-tpl-springboot-azure-basic",
	"vex-tpl-springboot-azure-commercial",
	"vex-tpl-springboot-azure-critical",
}

func TestPipelinecodeReal_PasaLaValidacionSinModificarse(t *testing.T) {
	for _, plantilla := range plantillasReales {
		t.Run(plantilla, func(t *testing.T) {
			path := filepath.Join(templatesPath, plantilla)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Skipf("el template %s no está en %s", plantilla, templatesPath)
			}

			ctx := context.Background()

			entries, err := infraPipeline.NewPipelineStepRepository().Get(&ctx, path)
			require.NoError(t, err)
			require.NotEmpty(t, entries, "el template tiene que declarar steps")

			require.NoError(t,
				domPipeline.NewPipelineStructureValidator().Validate(entries),
				"la estructura de steps del template tiene que ser válida sin tocarla")

			stepNames, err := domPipeline.NewStepNames(entries)
			require.NoError(t, err)
			assert.Equal(t,
				[]string{"01-test", "02-supply", "03-package", "04-deploy"},
				nombresDeSteps(stepNames),
				"y en este orden")

			// Y ningún ambiente se llama `shared` (§5.4). El nombre reservado se
			// comprueba en el handler 03; aquí lo que se fija es que el template no
			// lo declare.
			environments, err := infraPipeline.NewPipelineEnvironmentRepository().Get(&ctx, path)
			require.NoError(t, err)
			require.NotEmpty(t, environments)
			assert.NotContains(t, environments, command.SharedScopeName)

			// Un `commands.yaml` vacío daría `skipped{no_commands}` en vez de
			// ejecutar el step (§5.3): ninguno de los cuatro lo está.
			for _, stepName := range stepNames {
				commandsFile := filepath.Join(path, "steps", stepName.FullName(), "commands.yaml")
				info, err := os.Stat(commandsFile)
				require.NoError(t, err, "el step declara su directorio pero no su commands.yaml")
				assert.NotZero(t, info.Size(), "%s tiene el commands.yaml vacío", stepName.FullName())
			}
		})
	}
}

func nombresDeSteps(steps []command.StepName) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.FullName())
	}
	return out
}
