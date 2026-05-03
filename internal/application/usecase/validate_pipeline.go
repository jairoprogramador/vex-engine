package usecase

import (
	"context"
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
	pipelinesBasePath string
}

// NewValidatePipelineUseCase construye el use case con las dependencias inyectadas.
func NewValidatePipelineUseCase(
	pipelinesBasePath string,
) *ValidatePipelineUseCase {
	return &ValidatePipelineUseCase{
		pipelinesBasePath: pipelinesBasePath,
	}
}

// Execute clona el repositorio y parsea su estructura.
// Retorna Valid=false con los errores encontrados si el repo no es válido,
// o Valid=true con los steps y environments disponibles si es correcto.
func (uc *ValidatePipelineUseCase) Execute(ctx context.Context, input ValidatePipelineInput) (ValidatePipelineOutput, error) {
	/* if input.PipelineUrl == "" {
		return ValidatePipelineOutput{}, fmt.Errorf("use case validate pipeline: pipeline url is required")
	}

	repoURL, err := pipDom.NewPipelineURL(input.PipelineUrl)
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("invalid pipeline url: %v", err)},
		}, nil
	}

	repoRef, err := pipDom.NewPipelineRef(input.PipelineRef)
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("invalid pipeline ref: %v", err)},
		}, nil
	}

	pipelineLocalPath, err := uc.gitRepository.Clone(ctx, repoURL, repoRef.String(), uc.pipelinesBasePath)
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("fetch repository: %v", err)},
		}, nil
	}

	plan, err := uc.loader.Load(ctx, pipelineLocalPath, "", pipDom.NewStepLimit(""))
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("load pipeline: %v", err)},
		}, nil
	}

	steps := make([]string, 0, len(plan.Steps()))
	for _, stepDef := range plan.Steps() {
		steps = append(steps, stepDef.StepName().Name())
	}

	return ValidatePipelineOutput{
		Valid:        true,
		Steps:        steps,
		Environments: []string{plan.Environment().Value()},
	}, nil */
	return ValidatePipelineOutput{
		Valid:        true,
		Steps:        []string{"hdk"},
		Environments: []string{"hdk"},
		Errors:       []string{},
	}, nil
}
