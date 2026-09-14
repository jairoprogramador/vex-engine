package command

import (
	"context"
	"fmt"
)

type RegexCheckerHandler struct {
	CommandBaseHandler
}

var _ CommandHandler = (*RegexCheckerHandler)(nil)

func NewRegexCheckerHandler() CommandHandler {
	return &RegexCheckerHandler{CommandBaseHandler: CommandBaseHandler{Next: nil}}
}

func (h *RegexCheckerHandler) Handle(ctx *context.Context, request *CommandRequestHandler) error {
	for _, expectedOutput := range request.CommandOutputs() {
		matches := expectedOutput.CompiledProbe().FindStringSubmatch(request.CommandNormalizedStdout())
		if len(matches) < 1 {
			return fmt.Errorf("regex '%s' no encontró coincidencia en stdout del comando '%s'",
				expectedOutput.Probe(), request.CommandCmd())
		}
	}
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
