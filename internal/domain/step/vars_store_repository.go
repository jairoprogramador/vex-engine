package step

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type VarsStoreRepository interface {
	Get(ctx *context.Context, projectUrl, pipelineUrl, scope, step string) ([]command.Variable, error)
	Save(ctx *context.Context, projectUrl, pipelineUrl, scope, step string, vars []command.Variable) error
}
