package step

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// VarsStoreSharedHandler carga lo que este step dejó en el ámbito de PROYECTO:
// el trabajo común a todos los ambientes.
//
// Lee el ÚLTIMO registro de la clave, que es el estado vigente. Los anteriores
// siguen ahí y no los mira nadie todavía: son la historia de la que cuelgan el
// `evidence_from` de la spec 17 y el rollback de la 28, que sí eligen un
// registro concreto en vez del último.
type VarsStoreSharedHandler struct {
	StepBaseHandler
	records state.Records
}

var _ StepHandler = (*VarsStoreSharedHandler)(nil)

func NewVarsStoreSharedHandler(records state.Records) StepHandler {
	return &VarsStoreSharedHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		records:         records,
	}
}

func (h *VarsStoreSharedHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	key, err := request.ProjectStateKey()
	if err != nil {
		return fmt.Errorf("componer la clave de estado del proyecto: %w", err)
	}

	record, found, err := h.records.Last(ctx, key)
	if err != nil {
		return fmt.Errorf("cargar el estado del ámbito de proyecto: %w", err)
	}
	if !found {
		if h.Next != nil {
			return h.Next.Handle(ctx, request)
		}
		return nil
	}

	// Las variables se cargan TAL COMO se guardaron, marca de ámbito incluida.
	// Antes se reconstruían aquí con `shared=true` cableado, y era el adaptador
	// quien decidía si la marca sobrevivía al viaje: el de archivo la perdía y el
	// de Supabase la deducía del scope, así que el mismo proyecto producía una
	// huella distinta según dónde corriera (spec 02 §5.2).
	for _, variable := range record.Variables() {
		request.AddAccumulatedVars(variable)
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
