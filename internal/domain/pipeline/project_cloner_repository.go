package pipeline

import (
	"context"
)

type ProjectClonerRepository interface {
	Clone(ctx *context.Context, urlProject, refProject string) (string, error)
}
