package step

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

var variableInterpolationRegex = regexp.MustCompile(`\$\{var\.`)

type VarsHandler struct {
	StepBaseHandler
	repository VarsPipelineRepository
}

var _ StepHandler = (*VarsHandler)(nil)

func NewVarsHandler(varsRepository VarsPipelineRepository) StepHandler {
	return &VarsHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		repository:      varsRepository,
	}
}

func (h *VarsHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	variables, err := h.repository.Get(ctx, request.PipelineLocalPath(), request.Environment(), request.StepName())
	if err != nil {
		return fmt.Errorf("cargar vars pipeline: %w", err)
	}

	resolvedVars, err := h.Resolve(request.AccumulatedVars(), variables)
	if err != nil {
		return fmt.Errorf("resolver vars pipeline: %w", err)
	}

	request.AddAccumulatedVarsAll(resolvedVars)

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}

func (h *VarsHandler) Resolve(initialVars *command.ExecutionVariableMap, variablesToResolve []command.Variable) (*command.ExecutionVariableMap, error) {
	varsToResolve := command.NewExecutionVariableMap()

	for _, v := range *varsToResolve {
		variable, err := command.NewVariable(v.Name(), v.Value(), false)
		if err != nil {
			return nil, fmt.Errorf("crear variable de ejecución(pipeline): %w", err)
		}
		varsToResolve.Add(variable)
	}

	unresolvedVars := command.NewExecutionVariableMap()
	finalResolvedSet := []command.Variable{}
	for _, v := range *varsToResolve {
		if variableInterpolationRegex.MatchString(v.Value()) {
			unresolvedVars.Add(v)
		} else {
			finalResolvedSet = append(finalResolvedSet, v)
		}
	}

	if len(*unresolvedVars) == 0 {
		return varsToResolve, nil
	}

	varsForInterpolation := initialVars.Clone()
	varsForInterpolation.AddAll(finalResolvedSet)

	maxPasses := len(*unresolvedVars) + 1
	for pass := 0; pass < maxPasses; pass++ {
		if len(*unresolvedVars) == 0 {
			break
		}

		madeProgress := false
		varsStillUnresolved := command.NewExecutionVariableMap()

		for _, unresolvedVar := range *unresolvedVars {
			interpolatedValue, err := command.Interpolate(unresolvedVar.Value(), &varsForInterpolation)
			if err != nil {
				varsStillUnresolved.Add(unresolvedVar)
				continue
			}

			resolvedVar, err := command.NewVariable(unresolvedVar.Name(), interpolatedValue, false)
			if err != nil {
				return nil, fmt.Errorf("crear variable de ejecución(pipeline): %w", err)
			}
			finalResolvedSet = append(finalResolvedSet, resolvedVar)
			varsForInterpolation.Add(resolvedVar)
			madeProgress = true
		}

		unresolvedVars = varsStillUnresolved

		if !madeProgress {
			var missingVarNames []string
			for _, v := range *unresolvedVars {
				missingVarNames = append(missingVarNames, v.Name())
			}
			return nil, fmt.Errorf("dependencia circular o variable faltante detectada. No se pudieron resolver: %s", strings.Join(missingVarNames, ", "))
		}
	}

	if len(*unresolvedVars) > 0 {
		var missingVarNames []string
		for _, v := range *unresolvedVars {
			missingVarNames = append(missingVarNames, v.Name())
		}
		return nil, fmt.Errorf("no se pudieron resolver todas las variables. Faltantes: %s", strings.Join(missingVarNames, ", "))
	}

	resultVars := command.NewExecutionVariableMap()
	resultVars.AddAll(finalResolvedSet)
	return resultVars, nil
}
