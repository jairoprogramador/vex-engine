package step

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	"gopkg.in/yaml.v3"
)

// StepConfigFileName es el archivo donde un step declara lo que es. Se nombra
// aquí, y no en el dominio, porque es un detalle del formato en disco.
const StepConfigFileName = "config.yaml"

type pipelineStepConfigRepository struct{}

var _ domStep.StepConfigRepository = (*pipelineStepConfigRepository)(nil)

func NewPipelineStepConfigRepository() domStep.StepConfigRepository {
	return &pipelineStepConfigRepository{}
}

// Get distingue TRES situaciones, y las tres son distintas a propósito
// (spec 13 §5.3 y §7):
//
//   - el archivo no está        ⇒ `NoStepConfig()`, sin error. El step no
//     declara ámbito: se ejecutará siempre y no persistirá registro.
//   - el archivo está y declara ⇒ la configuración, con su ámbito.
//   - el archivo está y NO declara un ámbito del vocabulario cerrado ⇒ ERROR,
//     nombrando el directorio. Un `config.yaml` presente es una intención de
//     declarar, así que tratarlo como ausente sería adivinar cuál.
//
// El error se envuelve con la ruta relativa del archivo —no con la absoluta—
// porque quien lo lee edita el pipelinecode, no el directorio de trabajo que el
// motor clonó.
func (r *pipelineStepConfigRepository) Get(
	_ *context.Context, pipelineLocalPath, step string) (domStep.StepConfig, error) {

	relPath := filepath.ToSlash(filepath.Join("steps", step, StepConfigFileName))
	data, err := os.ReadFile(filepath.Join(pipelineLocalPath, "steps", step, StepConfigFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return domStep.NoStepConfig(), nil
		}
		return domStep.StepConfig{}, fmt.Errorf("leer '%s': %w", relPath, err)
	}

	var dto PipelineStepConfigDTO
	if err := yaml.Unmarshal(data, &dto); err != nil {
		return domStep.StepConfig{}, fmt.Errorf("parsear YAML de '%s': %w", relPath, err)
	}

	// Un archivo vacío entra por aquí con `Scope` vacío y sale como error, no
	// como ausencia: está presente, luego alguien quiso declarar algo.
	scope, err := domStep.NewScope(dto.Scope)
	if err != nil {
		return domStep.StepConfig{}, fmt.Errorf("'%s' %w", relPath, err)
	}
	return domStep.NewStepConfig(scope)
}
