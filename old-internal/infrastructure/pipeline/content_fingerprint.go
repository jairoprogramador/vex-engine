package pipeline

import (
	"fmt"

	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
	domPipeline "github.com/jairoprogramador/vex-engine/old-internal/domain/pipeline"
	infraFingerprint "github.com/jairoprogramador/vex-engine/old-internal/infrastructure/fingerprint"
)

var _ domPipeline.ContentFingerprint = (*ContentFingerprint)(nil)

// ContentFingerprint adapta la regla de huella del dominio a una ruta del
// sistema de archivos. Todo lo que hace es elegir la fuente de árbol: el
// cálculo no está aquí, y no debe estarlo.
type ContentFingerprint struct{}

func NewContentFingerprint() domPipeline.ContentFingerprint {
	return &ContentFingerprint{}
}

func (r *ContentFingerprint) FromDirectory(dirPath string) (domFingerprint.Fingerprint, error) {
	source, err := infraFingerprint.NewDirTreeSource(dirPath)
	if err != nil {
		return domFingerprint.Fingerprint{}, fmt.Errorf("content fingerprint: %w", err)
	}

	fp, err := domFingerprint.Compute(source)
	if err != nil {
		return domFingerprint.Fingerprint{}, fmt.Errorf(
			"content fingerprint: calcular la huella de %s: %w", dirPath, err)
	}

	return fp, nil
}
