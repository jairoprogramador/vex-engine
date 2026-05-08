package pipeline

import (
	"context"
	"fmt"
)

type CopyWorkdirHandler struct {
	PipelineBaseHandler
	repository PipelineWorkdirRepository
}

var _ PipelineHandler = (*CopyWorkdirHandler)(nil)

func NewCopyWorkdirHandler(repository PipelineWorkdirRepository) PipelineHandler {
	return &CopyWorkdirHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		repository:          repository,
	}
}

func (h *CopyWorkdirHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	pipelineWorkdir, err := h.repository.Copy(ctx, request.PipelineLocalPath(), request.ProjectUrl(), request.PipelineUrl(), request.Environment())
	if err != nil {
		return fmt.Errorf("copiar workdir: %w", err)
	}
	request.SetWorkdir(pipelineWorkdir)
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
