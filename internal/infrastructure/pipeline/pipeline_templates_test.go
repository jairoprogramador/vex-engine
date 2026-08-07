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
	infraStep "github.com/jairoprogramador/vex-engine/internal/infrastructure/step"
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

			configs := infraStep.NewPipelineStepConfigRepository()
			validador := domPipeline.NewPipelineStructureValidator(
				configs,
				infraPipeline.NewPipelineManifestRepository(),
				infraStep.NewPipelineVarsRepository(),
				infraStep.NewPipelineCommandRepository())

			// El ambiente entra en la validación desde la spec 14: las dos reglas de
			// la gramática leen `variables/<ambiente>/<paso>.yaml`. Los tres templates
			// declaran `sand`, y con él se comprueban.
			codigo := domPipeline.Pipelinecode{LocalPath: path, Environment: "sand"}
			require.NoError(t, validador.Validate(&ctx, codigo, entries),
				"la estructura de steps del template tiene que ser válida sin tocarla")

			stepNames, err := domPipeline.NewStepNames(entries)
			require.NoError(t, err)
			assert.Equal(t,
				[]string{"01-test", "02-supply", "03-package", "04-deploy"},
				nombresDeSteps(stepNames),
				"y en este orden")

			// Aquí se comprobaba que ningún ambiente se llamara `shared` (spec 04
			// §5.4). La spec 13 §5.5 DEROGA esa reserva —no queda ninguna palabra
			// reservada— así que el template ya no tiene nada que evitar.
			environments, err := infraPipeline.NewPipelineEnvironmentRepository().Get(&ctx, path)
			require.NoError(t, err)
			require.NotEmpty(t, environments)

			// EL ESTADO DE LOS TEMPLATES HOY, y es el residuo declarado de la
			// spec 13 §6: ninguno declara `config.yaml` todavía, así que sus steps se
			// ejecutan SIEMPRE y no persisten registro. Más lento que antes, nunca
			// incorrecto. Lo cierra la spec 24, que parte `02-supply` en dos steps
			// —uno de ámbito de proyecto para el ACR, otro de ambiente— y les pone su
			// declaración. Cuando llegue, este bloque se pone en rojo y la decisión se
			// hace visible.
			for _, stepName := range stepNames {
				config, err := configs.Get(&ctx, path, stepName.FullName())
				require.NoError(t, err, "un config.yaml presente tiene que declarar un ámbito válido")
				assert.False(t, config.IsDeclared(),
					"%s ya declara ámbito: llegó la spec 24 y este test tiene que decirlo",
					stepName.FullName())
			}

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
