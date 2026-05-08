package step

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type VarsStoreStepHandler struct {
	StepBaseHandler
	varsRepository VarsStoreRepository
}

var _ StepHandler = (*VarsStoreStepHandler)(nil)

func NewVarsStoreStepHandler(varsRepository VarsStoreRepository) StepHandler {
	return &VarsStoreStepHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		varsRepository:  varsRepository,
	}
}

func (h *VarsStoreStepHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	variables, err := h.varsRepository.Get(ctx, request.ProjectUrl(), request.PipelineUrl(), request.Environment(), request.StepNameExe())
	if err != nil {
		return fmt.Errorf("cargar vars step: %w", err)
	}
	for _, variable := range variables {
		commandVariable, err := command.NewVariable(variable.Name(), variable.Value(), false)
		if err != nil {
			return fmt.Errorf("crear variable de ejecución(step): %w", err)
		}
		request.AddAccumulatedVars(commandVariable)
	}
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
