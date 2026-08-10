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
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
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
				[]string{"01-test", "02-acr", "03-supply", "04-package", "05-deploy"},
				nombresDeSteps(stepNames),
				"y en este orden")

			// Aquí se comprobaba que ningún ambiente se llamara `shared` (spec 04
			// §5.4). La spec 13 §5.5 DEROGA esa reserva —no queda ninguna palabra
			// reservada— así que el template ya no tiene nada que evitar.
			environments, err := infraPipeline.NewPipelineEnvironmentRepository().Get(&ctx, path)
			require.NoError(t, err)
			require.NotEmpty(t, environments)

			// LA TESIS DE LA SPEC 24, medida sobre los archivos de verdad: los cinco
			// steps declaran su `config.yaml`, y cada uno declara UN ámbito.
			//
			// Este bloque afirmaba lo contrario hasta que la migración llegó —era la
			// forma de dejar escrito el residuo de la spec 13 §6— y cambia de signo
			// aquí. Lo que comprueba ahora son las cuatro decisiones de la spec 24
			// §5.2, que no son obvias ninguna: el ámbito de cada step, que los cinco
			// PERSISTEN (ámbito sin `rules` no escribe registro, y en esta migración
			// eso perdería los identificadores de recursos que ya existen en Azure) y
			// que ninguno pide la advertencia de la spec 15 §5.4.
			for _, esperado := range ambitosDeclarados {
				config, err := configs.Get(&ctx, path, esperado.step)
				require.NoError(t, err, "un config.yaml presente tiene que declarar un ámbito válido")

				require.True(t, config.IsDeclared(),
					"%s no declara config.yaml: la spec 24 exige que los cinco lo hagan",
					esperado.step)
				assert.Equal(t, esperado.scope, config.Scope().String(),
					"%s declara el ámbito equivocado", esperado.step)

				rules := config.Rules()
				assert.True(t, config.Remembers(),
					"%s declara ámbito y ninguna regla: se ejecutaría siempre SIN escribir registro",
					esperado.step)
				assert.False(t, rules.ExpiresWithoutInvalidating(),
					"%s caduca sin declarar qué lo invalida: la spec 15 §5.4 avisaría",
					esperado.step)

				stateChanged, declarada := rules.StateChanged()
				require.True(t, declarada, "%s no declara 'state_changed'", esperado.step)
				assert.Equal(t, esperado.watchesProject, stateChanged.WatchesProject(),
					"%s vigila el código del proyecto cuando no debía, o al revés", esperado.step)

				assert.Equal(t, esperado.expira, declara(rules, domStep.RuleKindMaxAge),
					"%s declara 'max_age' cuando no debía, o al revés", esperado.step)
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

// ambitosDeclarados es la spec 24 §5.2 escrita como tabla: qué declara cada step
// de los tres templates, y por qué.
//
//   - `01-test` es de ámbito de PROYECTO —el resultado de `mvn clean verify`
//     depende del código, no del ambiente— y es el ÚNICO que caduca: el TTL
//     global de 30 días se retiró (spec 15 §5.7), así que declararlo aquí y sólo
//     aquí es una decisión de estos templates, no lo que había.
//   - `02-acr` es de proyecto porque el registro de contenedores es uno para todo
//     el proyecto; no mira el código y no caduca.
//   - `03-supply` es de ambiente —el AKS y su grupo de recursos son de cada uno—
//     y tampoco mira el código: provisionar infraestructura no depende de que
//     cambie una clase Java.
//   - `04-package` y `05-deploy` sí lo miran, que es lo que la tabla cableada del
//     motor hacía antes de que el pipelinecode pudiera decirlo.
var ambitosDeclarados = []struct {
	step           string
	scope          string
	watchesProject bool
	expira         bool
}{
	{step: "01-test", scope: "project", watchesProject: true, expira: true},
	{step: "02-acr", scope: "project", watchesProject: false, expira: false},
	{step: "03-supply", scope: "environment", watchesProject: false, expira: false},
	{step: "04-package", scope: "environment", watchesProject: true, expira: false},
	{step: "05-deploy", scope: "environment", watchesProject: true, expira: false},
}

func declara(rules domStep.RuleSet, kind domStep.RuleKind) bool {
	for _, rule := range rules.Rules() {
		if rule.Kind() == kind {
			return true
		}
	}
	return false
}

func nombresDeSteps(steps []command.StepName) []string {
	out := make([]string, 0, len(steps))
	for _, s := range steps {
		out = append(out, s.FullName())
	}
	return out
}
