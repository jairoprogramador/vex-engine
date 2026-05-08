package step

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type VarsStoreSharedHandler struct {
	StepBaseHandler
	varsRepository VarsStoreRepository
}

var _ StepHandler = (*VarsStoreSharedHandler)(nil)

func NewVarsStoreSharedHandler(varsRepository VarsStoreRepository) StepHandler {
	return &VarsStoreSharedHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		varsRepository:  varsRepository,
	}
}

func (h *VarsStoreSharedHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	variables, err := h.varsRepository.Get(ctx, request.ProjectUrl(), request.PipelineUrl(), command.SharedScopeName, request.StepName())
	if err != nil {
		return fmt.Errorf("cargar vars shared: %w", err)
	}
	for _, variable := range variables {
		commandVariable, err := command.NewVariable(variable.Name(), variable.Value(), true)
		if err != nil {
			return fmt.Errorf("crear variable de ejecución(shared): %w", err)
		}
		request.AddAccumulatedVars(commandVariable)
	}
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
