package pipeline

import (
	"context"
	"fmt"
)

type PipelineClonerHandler struct {
	PipelineBaseHandler
	repository PipelineClonerRepository
}

var _ PipelineHandler = (*PipelineClonerHandler)(nil)

func NewPipelineClonerHandler(repository PipelineClonerRepository) PipelineHandler {
	return &PipelineClonerHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		repository:          repository,
	}
}

func (h *PipelineClonerHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	pipelineLocalPath, err := h.repository.Clone(ctx, request.ProjectUrl(), request.PipelineUrl(), request.PipelineRef())
	if err != nil {
		return fmt.Errorf("clonar pipeline: %w", err)
	}

	if pipelineLocalPath == "" {
		return fmt.Errorf("ruta local del pipeline no puede estar vacía")
	}

	request.SetPipelineLocalPath(pipelineLocalPath)
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
