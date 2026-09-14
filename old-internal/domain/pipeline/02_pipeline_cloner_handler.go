package pipeline

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

type PipelineClonerHandler struct {
	PipelineBaseHandler
	repository PipelineClonerRepository
	emitter    *record.Emitter
}

var _ PipelineHandler = (*PipelineClonerHandler)(nil)

func NewPipelineClonerHandler(
	repository PipelineClonerRepository, emitter *record.Emitter) PipelineHandler {

	return &PipelineClonerHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		repository:          repository,
		emitter:             emitter,
	}
}

// Handle elige la FUENTE del pipelinecode y deja dicho cuál eligió.
//
// Las tres políticas —clonar, reutilizar dentro de la ventana, o caer en el clon
// viejo porque el remoto no responde— salen del clonador con la misma forma: una
// ruta local y su procedencia (spec 18 §5.4). Aquí sólo se decide qué se hace con
// cada procedencia, y sólo una obliga a algo.
func (h *PipelineClonerHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	request.Emit("cloning pipeline")
	source, err := h.repository.Clone(ctx, request.PipelineUrl(), request.PipelineRef())
	if err != nil {
		return fmt.Errorf("clonar pipeline: %w", err)
	}

	if source.LocalPath == "" {
		return fmt.Errorf("ruta local del pipeline no puede estar vacía")
	}

	request.SetPipelineLocalPath(source.LocalPath)
	// El HEAD deja de descartarse: `git_clone.go` ya lo leía para verificar el
	// ref y lo tiraba. Viaja como METADATO del objeto, nunca como identidad
	// (§5.3): dos árboles idénticos con distinto commit de origen —un rebase, un
	// cherry-pick, dos clones de remotos distintos— tienen que producir el mismo
	// `content_id`, o la comparabilidad entre organizaciones desaparece.
	request.SetPipelineHeadHash(source.HeadHash)

	if source.IsStale() {
		// El hecho es OBLIGATORIO cuando ocurre, y no por auditoría sino por
		// identidad: la identidad se calcula sobre la fuente realmente usada, así
		// que sin este hecho un `content_id` afirmaría que se ejecutó una versión
		// del pipeline distinta de la que corrió y nadie podría saberlo.
		//
		// Se emite AQUÍ, que es donde se observa, aunque el `deployment_id` no
		// exista todavía: el emisor lo retiene hasta que el resolutor abre la tira
		// (spec 18 §5.1). Así `seq` refleja el orden en que las cosas pasaron y no
		// el orden en que se pudieron escribir.
		request.Emit(fmt.Sprintf(
			"advertencia: el remoto del pipelinecode no responde; se usa el clon local de hace %.1f h",
			source.AgeHours()))
		if err := h.emitter.Emit(ctx, record.StaleCloneUsed{
			Source:   request.PipelineUrl(),
			AgeHours: source.AgeHours(),
		}); err != nil {
			return fmt.Errorf("registrar el uso de un clon viejo: %w", err)
		}
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
