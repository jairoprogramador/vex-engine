package pipeline

import (
	"context"
	"fmt"
)

type PipelineRunnerHandler struct {
	PipelineBaseHandler
}

var _ PipelineHandler = (*PipelineRunnerHandler)(nil)

func NewPipelineRunnerHandler() PipelineHandler {
	return &PipelineRunnerHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
	}
}

func (h *PipelineRunnerHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {

	request.Emit("Iniciando la ejecución")
	request.Emit(fmt.Sprintf("  - Entorno: %s", request.Environment()))
	request.Emit(fmt.Sprintf("  - Versión: %s", request.ProjectVersion()))
	request.Emit(fmt.Sprintf("  - Commit: %s", request.ProjectHeadHash()))

	for _, stepName := range request.Steps() {
		request.SetStepName(stepName)
		if err := request.Execute(); err != nil {
			return err
		}
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
