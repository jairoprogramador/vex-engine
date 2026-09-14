package command

import (
	"context"
	"fmt"
)

type CommandInterpolatorHandler struct {
	CommandBaseHandler
}

var _ CommandHandler = (*CommandInterpolatorHandler)(nil)

func NewCommandInterpolatorHandler() CommandHandler {
	return &CommandInterpolatorHandler{
		CommandBaseHandler: CommandBaseHandler{Next: nil},
	}
}

func (h *CommandInterpolatorHandler) Handle(ctx *context.Context, request *CommandRequestHandler) error {
	command := request.CommandCmd()
	interpolated, err := Interpolate(command, request.AccumulatedVars())
	if err != nil {
		return fmt.Errorf("interpolar comando '%s': %w", command, err)
	}
	if interpolated == "" {
		return fmt.Errorf("comando interpolado vacío para comando '%s'", command)
	}
	request.SetCommandInterpolatedCmd(interpolated)
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
