package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	domPipeline "github.com/jairoprogramador/vex-engine/old-internal/domain/pipeline"
	"gopkg.in/yaml.v3"
)

// PipelineManifestFileName es el manifiesto en la RAÍZ del pipelinecode. Se
// nombra aquí, y no en el dominio, porque es un detalle del formato en disco —el
// mismo reparto que `StepConfigFileName`.
const PipelineManifestFileName = "vexpipeline.yaml"

// PipelineManifestDTO es la forma en disco.
//
// La versión es de TODO el formato, así que lo que crece con las specs
// siguientes son los archivos que la versión cubre y no este DTO. La ventana del
// clon es la excepción y no contradice la regla: no describe el formato, es un
// parámetro de OPERACIÓN que se declara aquí porque quien sabe con qué
// frecuencia cambia un pipeline es quien lo escribe (spec 18 §5.4).
type PipelineManifestDTO struct {
	SchemaVersion int `yaml:"schema_version"`

	// CloneWindow es una duración de Go (`24h`, `30m`). Se declara como texto y
	// no como un número de horas para que `15m` sea escribible: una ventana en
	// horas enteras obligaría a elegir entre `0` —que significa otra cosa— y una
	// hora entera para un pipelinecode que se está escribiendo.
	CloneWindow string `yaml:"clone_window"`
}

type pipelineManifestRepository struct{}

var _ domPipeline.ManifestRepository = (*pipelineManifestRepository)(nil)

func NewPipelineManifestRepository() domPipeline.ManifestRepository {
	return &pipelineManifestRepository{}
}

// Get distingue tres situaciones, como el lector del `config.yaml` de un step:
//
//   - el archivo no está        ⇒ `NoManifest()`, sin error: versión 1.
//   - el archivo está y declara ⇒ el manifiesto con su versión.
//   - el archivo está y declara una versión que este motor no entiende ⇒ ERROR,
//     nombrando el archivo.
func (r *pipelineManifestRepository) Get(
	_ *context.Context, pipelineLocalPath string) (domPipeline.Manifest, error) {

	data, err := os.ReadFile(filepath.Join(pipelineLocalPath, PipelineManifestFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return domPipeline.NoManifest(), nil
		}
		return domPipeline.Manifest{}, fmt.Errorf("leer '%s': %w", PipelineManifestFileName, err)
	}

	var dto PipelineManifestDTO
	if err := yaml.Unmarshal(data, &dto); err != nil {
		return domPipeline.Manifest{}, fmt.Errorf(
			"parsear YAML de '%s': %w", PipelineManifestFileName, err)
	}

	cloneWindow, err := cloneWindowOf(dto.CloneWindow)
	if err != nil {
		return domPipeline.Manifest{}, fmt.Errorf("'%s' %w", PipelineManifestFileName, err)
	}

	manifest, err := domPipeline.NewManifestWithCloneWindow(dto.SchemaVersion, cloneWindow)
	if err != nil {
		return domPipeline.Manifest{}, fmt.Errorf("'%s' %w", PipelineManifestFileName, err)
	}
	return manifest, nil
}

// cloneWindowOf traduce el texto a duración. Ausente es cero, que el dominio lee
// como «la de por defecto»; presente e ilegible es un ERROR, porque quien lo
// escribió tenía una intención y adivinarla sería peor que decírselo.
func cloneWindowOf(declared string) (time.Duration, error) {
	if strings.TrimSpace(declared) == "" {
		return 0, nil
	}
	window, err := time.ParseDuration(strings.TrimSpace(declared))
	if err != nil {
		return 0, fmt.Errorf(
			"declara 'clone_window: %s', que no es una duración (se espera '24h', '30m'): %w",
			declared, err)
	}
	return window, nil
}
