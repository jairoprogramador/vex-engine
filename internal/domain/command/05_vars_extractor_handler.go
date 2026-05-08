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

		executionVariable, err := NewVariable(name, value, isShared)
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
