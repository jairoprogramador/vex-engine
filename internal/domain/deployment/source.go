package deployment

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

// Source son las dos fuentes de las que sale un despliegue: el árbol del
// proyecto y el árbol del pipelinecode, cada uno por su huella (spec 08).
//
// # Una sola regla para los dos árboles
//
// Las dos huellas se calculan con la MISMA `fingerprint.Compute` sobre dos
// `DirTreeSource` distintos. No hay —ni debe haber— una variante «para el
// pipeline»: que las dos salgan de la misma regla es lo único que hace
// comparable un despliegue de una organización con el de otra.
//
// # La forma serializada es `v1:<hash>`, no `sha256:<hash>`
//
// El prefijo versiona la REGLA, no el algoritmo (spec 17 §8). El algoritmo no
// cambia entre v1 y v2; cambia qué entra en él. `Source` transporta la cadena
// canónica tal cual sale de `Fingerprint.String()`, sin reformatearla y sin
// partirla: si alguien guardara el hash pelado, dos huellas de reglas distintas
// pasarían a ser iguales.
//
// # Lo que NO entra: el commit
//
// El HEAD de cada clon es METADATO del objeto, no identidad (spec 18 §5.3). Dos
// árboles idénticos con distinto commit de origen —un rebase, un cherry-pick,
// dos clones de remotos distintos— deben tener el mismo `content_id`, porque si
// no la comparabilidad entre organizaciones desaparece. La huella identifica; el
// commit documenta.
type Source struct {
	project  fingerprint.Fingerprint
	pipeline fingerprint.Fingerprint
}

// NewSource compone la fuente. Las DOS huellas son obligatorias.
//
// Una fuente a medias no produce una identidad degradada: produce un error. Es
// la regla (d) de la spec 17 §5.1 —la misma que `cache.NewCacheKey`— y aquí
// pesa más, porque una colisión en el caché cuesta un paso saltado y una en el
// `content_id` cuesta dos despliegues distintos con la misma identidad, para
// siempre.
func NewSource(project, pipeline fingerprint.Fingerprint) (Source, error) {
	if project.IsZero() {
		return Source{}, fmt.Errorf("deployment: la fuente no tiene la huella del proyecto")
	}
	if pipeline.IsZero() {
		return Source{}, fmt.Errorf("deployment: la fuente no tiene la huella del pipelinecode")
	}
	return Source{project: project, pipeline: pipeline}, nil
}

// Project es la huella del árbol del código a desplegar.
func (s Source) Project() fingerprint.Fingerprint { return s.project }

// Pipeline es la huella del árbol del pipelinecode.
func (s Source) Pipeline() fingerprint.Fingerprint { return s.pipeline }

// IsZero indica que no hay fuente.
func (s Source) IsZero() bool { return s.project.IsZero() || s.pipeline.IsZero() }

// Equals es la regla de igualdad del value object.
func (s Source) Equals(other Source) bool {
	return s.project.Equals(other.project) && s.pipeline.Equals(other.pipeline)
}
