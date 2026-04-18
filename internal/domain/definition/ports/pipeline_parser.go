package ports

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/definition/aggregates"
)

// PipelineParser es responsable de cargar y ensamblar una definición de plan de ejecución completa.
type PipelineParser interface {
	Parser(ctx context.Context, pipelinePath, stepName, envName string) (*aggregates.ExecutionPlanDefinition, error)
}
