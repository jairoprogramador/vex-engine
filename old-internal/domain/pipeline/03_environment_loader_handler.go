package pipeline

import (
	"context"
	"fmt"
	"slices"
)

type EnvironmentLoaderHandler struct {
	PipelineBaseHandler
	repository PipelineEnvironmentRepository
}

var _ PipelineHandler = (*EnvironmentLoaderHandler)(nil)

func NewEnvironmentLoaderHandler(repository PipelineEnvironmentRepository) PipelineHandler {
	return &EnvironmentLoaderHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		repository:          repository,
	}
}

func (h *EnvironmentLoaderHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	request.Emit("loading environment")
	environments, err := h.repository.Get(ctx, request.PipelineLocalPath())
	if err != nil {
		return fmt.Errorf("cargar ambientes: %w", err)
	}

	if len(environments) == 0 {
		return fmt.Errorf("No hay ambientes configurados")
	}

	// AQUÍ se rechazaba un ambiente llamado `shared` (spec 04 §5.4). La spec 13
	// §5.5 DEROGA la reserva: el ámbito de ambiente viaja siempre prefijado en la
	// clave de estado —`environment:<nombre>`— así que un ambiente llamado
	// `shared` produce `environment:shared` y otro llamado `project` produce
	// `environment:project`; ninguno colisiona con el ámbito de proyecto. No
	// queda NINGUNA palabra reservada en el vocabulario del usuario.
	//
	// Lo que NO desaparece es la validación del nombre: `state.NewEnvironmentScope`
	// sigue rechazando `/`, `\`, `:`, `.` y `..`, que es lo que impide que un
	// `value: pro/duccion` produzca un directorio anidado dentro del almacén.
	// Aquella reserva prohibía un nombre para arreglar una colisión; ésta la
	// elimina cambiando la clave, que es la corrección que no le cuesta nada al
	// usuario.
	if request.Environment() == "" {
		request.SetEnvironment(environments[0])
	} else {
		contains := slices.Contains(environments, request.Environment())
		if !contains {
			return fmt.Errorf("el ambiente %s no está definido en el pipeline", request.Environment())
		}
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
