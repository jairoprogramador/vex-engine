package usecase

import (
	"context"
	"crypto/md5" //nolint:gosec // solo se usa para derivar un nombre de directorio, no para seguridad
	"fmt"

	defPrt "github.com/jairoprogramador/vex-engine/internal/domain/definition/ports"
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
	pipelineCloner defPrt.PipelineCloner
	pipelineParser defPrt.PipelineParser
}

// NewValidatePipelineUseCase construye el use case con las dependencias inyectadas.
func NewValidatePipelineUseCase(
	pipelineCloner defPrt.PipelineCloner,
	pipelineParser defPrt.PipelineParser,
) *ValidatePipelineUseCase {
	return &ValidatePipelineUseCase{
		pipelineCloner: pipelineCloner,
		pipelineParser: pipelineParser,
	}
}

// Execute clona el repositorio y parsea su estructura.
// Retorna Valid=false con los errores encontrados si el repo no es válido,
// o Valid=true con los steps y environments disponibles si es correcto.
func (uc *ValidatePipelineUseCase) Execute(ctx context.Context, pipelineInput ValidatePipelineInput) (ValidatePipelineOutput, error) {
	if pipelineInput.PipelineUrl == "" {
		return ValidatePipelineOutput{}, fmt.Errorf("use case validate pipeline: pipeline url is required")
	}

	localPath, err := uc.pipelineCloner.Clone(ctx, pipelineInput.PipelineUrl, pipelineInput.PipelineRef)
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("clone repository: %v", err)},
		}, nil
	}

	pipelineDefinition, err := uc.pipelineParser.Parser(ctx, localPath, "", "")
	if err != nil {
		return ValidatePipelineOutput{
			Valid:  false,
			Errors: []string{fmt.Sprintf("parse pipeline: %v", err)},
		}, nil
	}

	steps := make([]string, 0, len(pipelineDefinition.Steps()))
	for _, stepDef := range pipelineDefinition.Steps() {
		steps = append(steps, stepDef.NameDef().Name())
	}

	environments := []string{pipelineDefinition.Environment().String()}

	return ValidatePipelineOutput{
		Valid:        true,
		Steps:        steps,
		Environments: environments,
	}, nil
}

// urlHash retorna los primeros 8 caracteres del hash MD5 de la URL,
// para derivar un nombre de directorio único y seguro para el filesystem.
func urlHash(url string) string {
	//nolint:gosec // no es uso criptográfico — solo naming de directorios
	h := md5.Sum([]byte(url)) //nolint:gosec
	return fmt.Sprintf("%x", h)[:8]
}
