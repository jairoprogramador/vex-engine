package pipeline

import (
	"context"
)

// PipelineStepRepository lee los directorios de `steps/` y NADA más: no decide
// qué es un step válido —eso lo hace PipelineStructureValidator— ni en qué orden
// se ejecutan —eso lo hace NewStepNames— (spec 04 §5.2').
//
// Devuelve los nombres crudos precisamente para que el validador pueda nombrar
// en su error un directorio que no se pudo convertir en step.
type PipelineStepRepository interface {
	Get(ctx *context.Context, pipelineLocalPath string) ([]StepEntry, error)
}
