package services

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/domain/pipeline/ports"
)

type PlanResolver struct {
	pipelineReader ports.PipelineReader
}

func NewPlanResolver(pipelineReader ports.PipelineReader) *PlanResolver {
	return &PlanResolver{pipelineReader: pipelineReader}
}

// Load implementa la carga del pipeline a partir de un repositorio ya disponible localmente.
// pathBase es el identificador opaco retornado por RepositoryFetcher.Fetch().
// Cuando envName == "" retorna el primer entorno definido (comportamiento intencional:
// permite ejecutar sin especificar entorno usando el default del pipeline).
func (b *PlanResolver) Load(
	ctx context.Context,
	pathBase string,
	envName string,
	limit pipeline.StepLimit,
) (*pipeline.PipelinePlan, error) {
	environment, err := b.resolveEnvironment(ctx, pathBase, envName)
	if err != nil {
		return nil, err
	}

	stepsToExecute, err := b.resolveSteps(ctx, pathBase, limit)
	if err != nil {
		return nil, err
	}

	assembledSteps := make([]*pipeline.Step, 0, len(stepsToExecute))
	for _, stepName := range stepsToExecute {
		step, err := b.assembleStep(ctx, pathBase, stepName, environment)
		if err != nil {
			return nil, fmt.Errorf("ensamblar paso '%s': %w", stepName.Name(), err)
		}
		assembledSteps = append(assembledSteps, step)
	}

	return pipeline.NewPipelinePlan(environment, assembledSteps)
}

func (b *PlanResolver) resolveEnvironment(ctx context.Context, pathBase, envName string) (pipeline.Environment, error) {
	environments, err := b.pipelineReader.ReadEnvironments(ctx, pathBase)
	if err != nil {
		return pipeline.Environment{}, fmt.Errorf("no se pudieron leer los entornos: %w", err)
	}
	if len(environments) == 0 {
		return pipeline.Environment{}, errors.New("no hay entornos definidos en environments.yaml")
	}

	if envName == "" {
		return environments[0], nil
	}

	for _, env := range environments {
		if env.Value() == envName {
			return env, nil
		}
	}

	return pipeline.Environment{}, fmt.Errorf("el entorno '%s' no es válido", envName)
}

func (b *PlanResolver) resolveSteps(ctx context.Context, pathBase string, limit pipeline.StepLimit) ([]pipeline.StepName, error) {
	allStepNames, err := b.pipelineReader.ReadStepNames(ctx, pathBase)
	if err != nil {
		return nil, fmt.Errorf("no se pudieron leer los pasos: %w", err)
	}

	sort.Slice(allStepNames, func(i, j int) bool {
		return allStepNames[i].Order() < allStepNames[j].Order()
	})

	if limit.IsAll() {
		return allStepNames, nil
	}

	finalStepIndex := -1
	for i, s := range allStepNames {
		if s.Name() == limit.Name() {
			finalStepIndex = i
			break
		}
	}

	if finalStepIndex == -1 {
		return nil, fmt.Errorf("el paso final '%s' no se encontró", limit.Name())
	}

	return allStepNames[:finalStepIndex+1], nil
}

func (b *PlanResolver) assembleStep(
	ctx context.Context,
	pathBase string,
	stepName pipeline.StepName,
	env pipeline.Environment,
) (*pipeline.Step, error) {
	commands, err := b.pipelineReader.ReadCommands(ctx, pathBase, stepName)
	if err != nil {
		return nil, fmt.Errorf("leer comandos: %w", err)
	}

	variables, err := b.pipelineReader.ReadVariables(ctx, pathBase, env, stepName)
	if err != nil {
		return nil, fmt.Errorf("leer variables: %w", err)
	}

	return pipeline.NewStep(stepName, commands, variables)
}
