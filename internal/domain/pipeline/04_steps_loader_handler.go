package pipeline

import (
	"context"
	"fmt"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"slices"
)

type StepsLoaderHandler struct {
	PipelineBaseHandler
	repository PipelineStepRepository
}

var _ PipelineHandler = (*StepsLoaderHandler)(nil)

func NewStepsLoaderHandler(repository PipelineStepRepository) PipelineHandler {
	return &StepsLoaderHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		repository:          repository,
	}
}

func (h *StepsLoaderHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	stepNames, err := h.repository.Get(ctx, request.PipelineLocalPath())
	if err != nil {
		return fmt.Errorf("cargar pasos: %w", err)
	}

	contains := slices.ContainsFunc(stepNames, func(stepName command.StepName) bool {
		return stepName.Name() == request.StepName()
	})

	if !contains {
		return fmt.Errorf("el paso %s no está definido en el pipeline", request.StepName())
	}

	stepIndex := slices.IndexFunc(stepNames, func(stepName command.StepName) bool {
		return stepName.Name() == request.StepName()
	})

	request.SetSteps(stepNames[:stepIndex+1])
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
