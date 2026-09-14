package step

import (
	"context"
)

type StepHandler interface {
	SetNext(next StepHandler)
	Handle(ctx *context.Context, request *StepRequestHandler) error
}
