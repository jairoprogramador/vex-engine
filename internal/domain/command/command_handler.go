package command

import (
	"context"
)

type CommandHandler interface {
	SetNext(next CommandHandler)
	Handle(ctx *context.Context, request *CommandRequestHandler) error
}
