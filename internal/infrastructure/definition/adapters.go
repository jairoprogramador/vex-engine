package definition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	defAgg "github.com/jairoprogramador/vex-engine/internal/domain/definition/aggregates"
	defPrt "github.com/jairoprogramador/vex-engine/internal/domain/definition/ports"
	defSvc "github.com/jairoprogramador/vex-engine/internal/domain/definition/services"
	proPrt "github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
)

// PipelineParserAdapter adapta *defSvc.PlanBuilder a la interfaz defPrt.PipelineParser.
// PlanBuilder expone Build(); PipelineParser requiere Parser() — este adapter cierra la brecha.
type PipelineParserAdapter struct {
	planBuilder *defSvc.PlanBuilder
}

// NewPipelineParserAdapter construye el adapter con el PlanBuilder inyectado.
func NewPipelineParserAdapter(planBuilder *defSvc.PlanBuilder) defPrt.PipelineParser {
	return &PipelineParserAdapter{planBuilder: planBuilder}
}

// Parser implementa defPrt.PipelineParser delegando a PlanBuilder.Build.
func (a *PipelineParserAdapter) Parser(
	ctx context.Context,
	pipelinePath, stepName, envName string,
) (*defAgg.ExecutionPlanDefinition, error) {
	plan, err := a.planBuilder.Build(ctx, pipelinePath, stepName, envName)
	if err != nil {
		return nil, fmt.Errorf("pipeline parser adapter: %w", err)
	}
	return plan, nil
}

var _ defPrt.PipelineParser = (*PipelineParserAdapter)(nil)

// PipelineClonerAdapter adapta proPrt.ClonerTemplate a la interfaz defPrt.PipelineCloner.
// ClonerTemplate requiere un localPath explícito; PipelineCloner lo deriva de la URL.
type PipelineClonerAdapter struct {
	cloner   proPrt.ClonerTemplate
	basePath string
}

// NewPipelineClonerAdapter construye el adapter. basePath es el directorio raíz donde
// se alojan los repositorios clonados (e.g. /var/lib/vexd/pipelines).
func NewPipelineClonerAdapter(cloner proPrt.ClonerTemplate, basePath string) defPrt.PipelineCloner {
	return &PipelineClonerAdapter{cloner: cloner, basePath: basePath}
}

// Clone clona el repositorio en basePath/<dir-derivado-de-url> y retorna el path local.
func (a *PipelineClonerAdapter) Clone(ctx context.Context, repositoryURL, ref string) (string, error) {
	dirName := urlSafeDir(repositoryURL)
	localPath := filepath.Join(a.basePath, dirName)

	if err := os.MkdirAll(a.basePath, 0o750); err != nil {
		return "", fmt.Errorf("pipeline cloner adapter: crear directorio base: %w", err)
	}

	if err := a.cloner.EnsureCloned(ctx, repositoryURL, ref, localPath); err != nil {
		return "", fmt.Errorf("pipeline cloner adapter: %w", err)
	}

	return localPath, nil
}

var _ defPrt.PipelineCloner = (*PipelineClonerAdapter)(nil)

// urlSafeDir deriva un nombre de directorio seguro a partir de una URL.
// Toma los últimos dos segmentos del path para mantener legibilidad.
func urlSafeDir(rawURL string) string {
	// Usa el último segmento de la ruta como nombre de directorio.
	base := filepath.Base(rawURL)
	if base == "." || base == "/" {
		base = "pipeline"
	}
	// Elimina extensión .git si la tiene.
	if ext := filepath.Ext(base); ext == ".git" {
		base = base[:len(base)-len(ext)]
	}
	return base
}
