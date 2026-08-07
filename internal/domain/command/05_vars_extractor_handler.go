package command

import (
	"context"
	"fmt"
)

type VarsExtractorHandler struct {
	CommandBaseHandler
}

var _ CommandHandler = (*VarsExtractorHandler)(nil)

func NewVarsExtractorHandler() CommandHandler {
	return &VarsExtractorHandler{
		CommandBaseHandler: CommandBaseHandler{Next: nil},
	}
}

func (h *VarsExtractorHandler) Handle(ctx *context.Context, request *CommandRequestHandler) error {
	vars, err := ExtractVars(request.CommandNormalizedStdout(), request.CommandOutputs())
	if err != nil {
		return fmt.Errorf("extraer variables de output: %w", err)
	}

	// Aquí se leía `request.CommandWorkdirIsShared()` para marcar cada variable
	// con el ámbito deducido del primer segmento del workdir. Lo retira la
	// spec 13 §5.6: el ámbito es del STEP, así que todas las variables que este
	// comando produce ya lo tienen y la marca por variable sobraba.
	for name, value := range vars {
		commandVariable, err := NewCommandVariable(name, value)
		if err != nil {
			return fmt.Errorf("crear variable de comando: %w", err)
		}
		request.AddCommandVar(commandVariable)

		// `OriginRuntime`: lo que el mundo real devolvió al ejecutar. Es la
		// precedencia más alta, y por eso un literal declarado con el mismo nombre
		// pasa a ser lo que dice ser, un valor por defecto (spec 12 §5.1).
		executionVariable, err := NewVariable(name, value, OriginRuntime)
		if err != nil {
			return fmt.Errorf("crear variable de ejecución: %w", err)
		}
		request.AddAccumulatedVars(executionVariable)

		// Y se anota como PRODUCIDA por este step. Son dos destinos porque son dos
		// preguntas distintas: el mapa acumulado responde «¿qué ve el step
		// siguiente?» y esto responde «¿qué dejó éste?», que es lo único que su
		// registro debe guardar (spec 14 §6). Hasta ahora el registro guardaba el
		// mapa acumulado entero, así que un literal declarado volvía del almacén
		// como `OriginState` en la corrida siguiente y editarlo dejaba de surtir
		// efecto.
		request.AddProducedVar(executionVariable)
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
