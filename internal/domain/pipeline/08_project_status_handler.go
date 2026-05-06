package pipeline

import (
	"context"
	"fmt"
)

type ProjectStatusHandler struct {
	PipelineBaseHandler
	fingerprint ProjectFingerprint
}

var _ PipelineHandler = (*ProjectStatusHandler)(nil)

func NewProjectStatusHandler(fingerprint ProjectFingerprint) PipelineHandler {
	return &ProjectStatusHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		fingerprint:         fingerprint,
	}
}

func (h *ProjectStatusHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	request.NotifyStage("computing_fingerprint")
	statusFingerprint, err := h.fingerprint.FromDirectory(request.ProjectLocalPath())
	if err != nil {
		return fmt.Errorf("obtener fingerprint de estado del proyecto: %w", err)
	}

	request.SetProjectStatus(statusFingerprint)

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
