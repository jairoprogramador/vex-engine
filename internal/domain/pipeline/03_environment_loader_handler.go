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
	request.NotifyStage("loading_environment")
	environments, err := h.repository.Get(ctx, request.PipelineLocalPath())
	if err != nil {
		return fmt.Errorf("cargar ambientes: %w", err)
	}

	if len(environments) == 0 {
		return fmt.Errorf("No hay ambientes configurados")
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
