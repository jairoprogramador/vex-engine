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

	isShared := request.CommandWorkdirIsShared()

	for name, value := range vars {
		commandVariable, err := NewCommandVariable(name, value, isShared)
		if err != nil {
			return fmt.Errorf("crear variable de comando: %w", err)
		}
		request.AddCommandVar(commandVariable)

		// `OriginRuntime`: lo que el mundo real devolvió al ejecutar. Es la
		// precedencia más alta, y por eso un literal declarado con el mismo nombre
		// pasa a ser lo que dice ser, un valor por defecto (spec 12 §5.1).
		executionVariable, err := NewVariable(name, value, isShared, OriginRuntime)
		if err != nil {
			return fmt.Errorf("crear variable de ejecución: %w", err)
		}
		request.AddAccumulatedVars(executionVariable)
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
