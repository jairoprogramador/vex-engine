package step

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// VarsStoreStepHandler carga lo que este step dejó en el ámbito del AMBIENTE en
// ejecución.
//
// Corre DESPUÉS del handler 01 y ANTES del 03: lo declarado por el pipelinecode
// gana sobre lo almacenado, y el ámbito de proyecto se carga antes que el del
// ambiente. Los dos órdenes son intencionados y hay canarios que los fijan.
type VarsStoreStepHandler struct {
	StepBaseHandler
	records state.Records
}

var _ StepHandler = (*VarsStoreStepHandler)(nil)

func NewVarsStoreStepHandler(records state.Records) StepHandler {
	return &VarsStoreStepHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		records:         records,
	}
}

func (h *VarsStoreStepHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	key, err := request.EnvironmentStateKey()
	if err != nil {
		return fmt.Errorf("componer la clave de estado del ambiente: %w", err)
	}

	record, found, err := h.records.Last(ctx, key)
	if err != nil {
		return fmt.Errorf("cargar el estado del ámbito de ambiente: %w", err)
	}
	if !found {
		if h.Next != nil {
			return h.Next.Handle(ctx, request)
		}
		return nil
	}

	for _, variable := range record.Variables() {
		request.AddAccumulatedVars(variable)
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
