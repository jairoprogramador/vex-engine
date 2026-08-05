package pipeline

import (
	"context"
	"fmt"
	"slices"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
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

	// `shared` es el ámbito del almacén compartido, y el nombre del ambiente
	// ocupa esa misma posición en la ruta `store/<pipeline>/<ámbito>/`. Un
	// ambiente así declarado pisaría el almacén compartido; la spec 11 lo
	// convierte además en una colisión de claves de estado (spec 04 §5.4).
	if slices.Contains(environments, command.SharedScopeName) {
		return fmt.Errorf(
			"el ambiente '%s' está reservado: environments.yaml no puede declararlo como value",
			command.SharedScopeName)
	}

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
