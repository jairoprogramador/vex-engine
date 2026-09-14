package step

import (
	"context"
	"fmt"
	"path/filepath"

	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	domStep "github.com/jairoprogramador/vex-engine/old-internal/domain/step"
	infraFingerprint "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/fingerprint"
)

var _ domStep.StepTreeRepository = (*PipelineStepTreeRepository)(nil)

// PipelineStepTreeRepository sirve el árbol del directorio de un step del
// pipelinecode: `<clon>/steps/<NN-nombre>/`.
//
// Es el segundo `DirTreeSource` con raíz propia —el primero es el del proyecto,
// y la spec 18 añadió el del pipelinecode entero— y los tres pasan por la MISMA
// regla de árbol sobre el mismo puerto. Servir las tres raíces por la misma
// `Compute` es lo único que impide que alguien introduzca una variante «para el
// pipelinecode», que es como nacen dos reglas que divergen (spec 27 §5.2').
//
// El directorio SIEMPRE existe cuando esto se llama: los steps de la operación
// salen de recorrer `steps/`, así que un `Get` que no lo encuentre es un defecto
// del motor y no un pipelinecode mal escrito. Se dice como error y no se degrada
// a un árbol vacío: un árbol vacío es una huella perfectamente válida que
// COLISIONA con la de cualquier step al que le falte lo mismo.
type PipelineStepTreeRepository struct{}

func NewPipelineStepTreeRepository() domStep.StepTreeRepository {
	return &PipelineStepTreeRepository{}
}

func (r *PipelineStepTreeRepository) Get(
	_ *context.Context, pipelineLocalPath, step string) (domFingerprint.TreeSource, error) {

	root := filepath.Join(pipelineLocalPath, "steps", step)
	source, err := infraFingerprint.NewDirTreeSource(root)
	if err != nil {
		return nil, fmt.Errorf("árbol del step '%s': %w", step, err)
	}
	return source, nil
}
