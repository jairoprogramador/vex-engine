package ports

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
)

// PipelineReader es la interfaz de lectura de datos que el PlanBuilder requiere.
// Se define en el lado del consumidor para mantener la testabilidad.
// pathBase es opaco para el dominio — la implementación concreta construye sus rutas internamente.
type PipelineReader interface {
	ReadEnvironments(ctx context.Context, pathBase string) ([]pipeline.Environment, error)
	ReadStepNames(ctx context.Context, pathBase string) ([]pipeline.StepName, error)
	ReadCommands(ctx context.Context, pathBase string, stepName pipeline.StepName) ([]pipeline.Command, error)
	ReadVariables(ctx context.Context, pathBase string, env pipeline.Environment, stepName pipeline.StepName) ([]pipeline.Variable, error)
}
