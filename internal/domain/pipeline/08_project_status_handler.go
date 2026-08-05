package pipeline

import (
	"context"
	"fmt"
)

type ProjectStatusHandler struct {
	PipelineBaseHandler
	fingerprint ContentFingerprint
}

var _ PipelineHandler = (*ProjectStatusHandler)(nil)

func NewProjectStatusHandler(fingerprint ContentFingerprint) PipelineHandler {
	return &ProjectStatusHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		fingerprint:         fingerprint,
	}
}

func (h *ProjectStatusHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	request.Emit("computing fingerprint")
	statusFingerprint, err := h.fingerprint.FromDirectory(request.ProjectLocalPath())
	if err != nil {
		return fmt.Errorf("obtener fingerprint de estado del proyecto: %w", err)
	}

	// La forma canónica lleva el prefijo de versión de la regla ("v1:"): lo que
	// se compara y se persiste aguas abajo es esa cadena, no el hash pelado.
	request.SetProjectStatus(statusFingerprint.String())

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
