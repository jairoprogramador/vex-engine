package pipeline

import (
	"context"
	"fmt"
	"slices"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type StepsLoaderHandler struct {
	PipelineBaseHandler
	repository PipelineStepRepository
	validator  PipelineStructureValidator
}

var _ PipelineHandler = (*StepsLoaderHandler)(nil)

func NewStepsLoaderHandler(
	repository PipelineStepRepository,
	validator PipelineStructureValidator) PipelineHandler {

	return &StepsLoaderHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		repository:          repository,
		validator:           validator,
	}
}

func (h *StepsLoaderHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	request.Emit("loading steps")
	entries, err := h.repository.Get(ctx, request.PipelineLocalPath())
	if err != nil {
		return fmt.Errorf("cargar pasos: %w", err)
	}

	// La estructura se valida ENTERA antes de construir un solo step: es el
	// único punto donde el pipelinecode entra al modelo, y el único momento en
	// que fallar no deja efectos a medias en la nube (spec 04 §5.1).
	if err := h.validator.Validate(entries); err != nil {
		return fmt.Errorf("estructura del pipelinecode inválida: %w", err)
	}

	stepNames, err := NewStepNames(entries)
	if err != nil {
		return fmt.Errorf("cargar pasos: %w", err)
	}

	stepIndex := slices.IndexFunc(stepNames, func(stepName command.StepName) bool {
		return stepName.Name() == request.StepName()
	})

	if stepIndex < 0 {
		return fmt.Errorf("el paso %s no está definido en el pipeline", request.StepName())
	}

	request.SetSteps(stepNames[:stepIndex+1])
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
