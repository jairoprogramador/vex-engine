package pipeline

import (
	"context"
	"fmt"
)

type ProjectClonerHandler struct {
	PipelineBaseHandler
	repository ProjectClonerRepository
}

var _ PipelineHandler = (*ProjectClonerHandler)(nil)

func NewProjectClonerHandler(repository ProjectClonerRepository) PipelineHandler {
	return &ProjectClonerHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		repository:          repository,
	}
}

func (h *ProjectClonerHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	request.NotifyStage("cloning_project")
	projectLocalPath, err := h.repository.Clone(ctx, request.ProjectUrl(), request.ProjectRef())
	if err != nil {
		return fmt.Errorf("clonar proyecto: %w", err)
	}

	if projectLocalPath == "" {
		return fmt.Errorf("ruta local del proyecto no puede estar vacía")
	}

	request.SetProjectLocalPath(projectLocalPath)
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
