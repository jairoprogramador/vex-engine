package pipeline

import (
	"context"
	"time"
)

// DefaultCloneWindow es cuánto tiempo se considera válido un clon del
// pipelinecode antes de volver a traerlo (spec 18 §5.4, cierra A5/BL-14).
//
// 24 h porque coincide con el ciclo de trabajo diario. Es un PARÁMETRO y no una
// estructura, y se puede cambiar desde el pipelinecode (`clone_window` en
// `vexpipeline.yaml`) porque quien sabe con qué frecuencia cambia un pipeline es
// quien lo escribe.
const DefaultCloneWindow = 24 * time.Hour

// CloneOrigin es de dónde salió el clon que se va a usar. Es la salida común de
// las tres políticas de resolución de la fuente (§5.3', Strategy).
//
// Existe como DATO y no como un `if` anidado dentro del clonador por una razón
// concreta: el tercer caso tiene que emitir un hecho, y una condición escondida
// en el flujo de control no obliga a nadie a recordarlo. Con la procedencia en la
// salida, el handler tiene que decidir qué hace con cada valor.
type CloneOrigin string

const (
	// CloneFresh es el clon recién traído del remoto.
	CloneFresh CloneOrigin = "fresh"

	// CloneReused es el clon que ya estaba y sigue dentro de su ventana. No se
	// tocó la red.
	CloneReused CloneOrigin = "reused"

	// CloneStale es el clon viejo que se usó PORQUE EL REMOTO NO RESPONDIÓ. Es
	// el único que obliga a emitir `stale_clone_used`: sin ese hecho, la
	// identidad afirmaría que se ejecutó una versión del pipeline distinta de la
	// que corrió y nadie podría saberlo.
	CloneStale CloneOrigin = "stale"
)

// PipelineSource es la fuente del pipelinecode REALMENTE USADA.
//
// La regla que une la ventana de reutilización con la identidad es que **la
// identidad se calcula sobre la fuente realmente usada, nunca sobre la que se
// pidió** (spec 18 §2). Por eso el clonador devuelve procedencia y no sólo una
// ruta: quien compone el objeto necesita saber contra qué árbol lo compone, y
// quien registra necesita poder explicarlo.
type PipelineSource struct {
	// LocalPath es el árbol de trabajo del pipelinecode en disco.
	LocalPath string

	// HeadHash es el commit del clon. Es METADATO, no identidad: la huella
	// identifica, el commit documenta (§5.3). Dos árboles idénticos con distinto
	// commit de origen —un rebase, un cherry-pick, dos clones de remotos
	// distintos— tienen el mismo `content_id`, y sin eso la comparabilidad entre
	// organizaciones desaparece.
	HeadHash string

	// Origin es cuál de las tres políticas resolvió esta fuente.
	Origin CloneOrigin

	// Age es la antigüedad del clon. Cero cuando se acaba de traer.
	Age time.Duration
}

// IsStale dice si esta fuente es un clon viejo usado por fallo del remoto, y por
// tanto un hecho que hay que registrar.
func (s PipelineSource) IsStale() bool { return s.Origin == CloneStale }

// AgeHours es la antigüedad en horas, que es la unidad del hecho
// `stale_clone_used`.
func (s PipelineSource) AgeHours() float64 { return s.Age.Hours() }

// PipelineClonerRepository resuelve de dónde sale el pipelinecode de esta
// ejecución.
//
// Devolvía una ruta hasta la spec 18, y con ella se perdían las dos cosas que
// esta spec necesita: cuál de las tres políticas la produjo, y el commit del
// árbol. Además hacía `os.RemoveAll` + clon en CADA ejecución, sin ventana y sin
// fallback: si el remoto no respondía, la ejecución fallaba aunque hubiera un
// clon perfectamente válido en disco (BL-27).
type PipelineClonerRepository interface {
	Clone(ctx *context.Context, urlPipeline, refPipeline string) (PipelineSource, error)
}
