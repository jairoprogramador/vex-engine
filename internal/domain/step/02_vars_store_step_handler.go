package step

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// VarsStoreStepHandler carga lo que este step dejó en el ámbito del AMBIENTE en
// ejecución.
//
// Corre DESPUÉS del handler 01, y eso ya no decide nada: desde la spec 12 los
// dos ámbitos comparten posición en el enum `command.Origin`, así que el que
// carga segundo gana por la regla de igualdad de `Add`, no por el cableado. El
// orden de carga se conserva —proyecto, luego ambiente— porque es el de §5.3 y
// porque lo específico debe llegar después de lo común.
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
