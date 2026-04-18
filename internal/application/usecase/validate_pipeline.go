package usecase

import (
	"context"
	"fmt"

	pipDom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	pipPrt "github.com/jairoprogramador/vex-engine/internal/domain/pipeline/ports"
	pipSer "github.com/jairoprogramador/vex-engine/internal/domain/pipeline/services"
)

// ValidatePipelineInput contiene los parámetros para validar un repositorio pipeline.
type ValidatePipelineInput struct {
	PipelineUrl string
	PipelineRef string
}

// ValidatePipelineOutput reporta si el repositorio pipeline es parseable.
type ValidatePipelineOutput struct {
	Valid        bool
	Steps        []string
	Environments []string
	Errors       []string
}

// ValidatePipelineUseCase clona el repositorio pipeline y valida su estructura
// sin ejecutar ningún paso.
type ValidatePipelineUseCase struct {
	fetcher pipPrt.RepositoryFetcher
	loader  *pipSer.PlanBuilder
}

// NewValidatePipelineUseCase construye el use case con las dependencias inyectadas.
func NewValidatePipelineUseCase(
	fetcher pipPrt.RepositoryFetcher,
	loader *pipSer.PlanBuilder,
) *ValidatePipelineUseCase {
	return &ValidatePipelineUseCase{
		fetcher: fetcher,
		loader:  loader,
	}
}

// Execute clona el repositorio y parsea su estructura.
// Retorna Valid=false con los errores encontrados si el repo no es válido,
// o Valid=true con los steps y environments disponibles si es correcto.
func (uc *ValidatePipelineUseCase) Execute(ctx context.Context, input ValidatePipelineInput) (ValidatePipelineOutput, error) {
	if input.PipelineUrl == "" {
		return ValidatePipelineOutput{}, fmt.Errorf("use case validate pipeline: pipeline url is required")
	}

	repoURL, err := pipDom.NewRepositoryURL(input.PipelineUrl)
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("invalid pipeline url: %v", err)},
		}, nil
	}

	repoRef, err := pipDom.NewRepositoryRef(input.PipelineRef)
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("invalid pipeline ref: %v", err)},
		}, nil
	}

	localPath, err := uc.fetcher.Fetch(ctx, repoURL, repoRef)
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("fetch repository: %v", err)},
		}, nil
	}

	plan, err := uc.loader.Load(ctx, localPath, "", pipDom.NewStepLimit(""))
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("load pipeline: %v", err)},
		}, nil
	}

	steps := make([]string, 0, len(plan.Steps()))
	for _, stepDef := range plan.Steps() {
		steps = append(steps, stepDef.Name().Name())
	}

	return ValidatePipelineOutput{
		Valid:        true,
		Steps:        steps,
		Environments: []string{plan.Environment().Value()},
	}, nil
}
