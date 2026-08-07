package step

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

var variableInterpolationRegex = regexp.MustCompile(`\$\{var\.`)

// VarsHandler convierte lo que el pipelinecode DECLARA en valores del mapa
// acumulado.
//
// Son dos trabajos distintos desde la spec 14, y el orden entre ellos importa:
//
//  1. las declaradas CON FUENTE (`resolve`) se satisfacen preguntándole a su
//     fuente, que es un sitio concreto y nombrado;
//  2. los LITERALES se resuelven interpolando `${var.…}` contra el mapa
//     acumulado, como siempre.
//
// Primero las fuentes, porque un literal puede interpolar una variable resuelta
// por declaración y al revés no: una declaración nombra su fuente, no la deduce
// del mapa.
type VarsHandler struct {
	StepBaseHandler
	repository VarsPipelineRepository
	resolvers  DeclarationResolvers
}

var _ StepHandler = (*VarsHandler)(nil)

func NewVarsHandler(varsRepository VarsPipelineRepository, resolvers DeclarationResolvers) StepHandler {
	return &VarsHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		repository:      varsRepository,
		resolvers:       resolvers,
	}
}

func (h *VarsHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	declarations, err := h.repository.Get(ctx, request.PipelineLocalPath(), request.Environment(), request.StepName())
	if err != nil {
		return fmt.Errorf("cargar vars pipeline: %w", err)
	}

	literals, sourced := partitionDeclarations(declarations)

	// Las declaraciones con fuente se anotan ANTES de resolverse: lo que entra en
	// la identidad del step es la declaración, la resuelva bien o mal (§5.3).
	request.SetSourcedDeclarations(sourced)

	for _, declaration := range sourced {
		value, err := h.resolvers.Resolve(ctx, request, declaration)
		if err != nil {
			return err
		}
		variable, err := command.NewVariable(declaration.Name(), value, command.OriginResolved)
		if err != nil {
			return fmt.Errorf("crear variable resuelta por declaración: %w", err)
		}
		request.AddAccumulatedVars(variable)
	}

	resolvedVars, err := h.Resolve(request.AccumulatedVars(), literals)
	if err != nil {
		return fmt.Errorf("resolver vars pipeline: %w", err)
	}

	request.AddAccumulatedVarsAll(resolvedVars)

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}

// partitionDeclarations separa los dos casos de la gramática. No es un detalle
// de implementación: es el reparto que la spec introduce —lo que una persona
// escribió frente a lo que otra parte del sistema producirá— y hasta ahora no
// existía porque las dos cosas llegaban como el mismo `{nombre, valor}`.
func partitionDeclarations(declarations []VariableDeclaration) (literals, sourced []VariableDeclaration) {
	literals = make([]VariableDeclaration, 0, len(declarations))
	sourced = make([]VariableDeclaration, 0)
	for _, declaration := range declarations {
		if declaration.IsLiteral() {
			literals = append(literals, declaration)
			continue
		}
		sourced = append(sourced, declaration)
	}
	return literals, sourced
}

// Resolve interpola los literales entre sí y contra el mapa acumulado.
//
// Sigue siendo lo de siempre, y sigue necesitando que el almacén se haya cargado
// antes: un literal puede interpolar un nombre que sólo vive en el registro del
// propio step (spec 12 §9.1). Lo que cambia es que ahora un fallo de
// interpolación es sólo eso —un literal que no cuadra—, porque lo que llega de
// otra parte del sistema ya no pasa por aquí: lo declara y lo resuelve su fuente.
func (h *VarsHandler) Resolve(initialVars *command.ExecutionVariableMap, declarations []VariableDeclaration) (*command.ExecutionVariableMap, error) {
	varsToResolve := command.NewExecutionVariableMap()

	for _, declaration := range declarations {
		variable, err := command.NewVariable(declaration.Name(), declaration.Value(), command.OriginDeclared)
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

			resolvedVar, err := command.NewVariable(
				unresolvedVar.Name(), interpolatedValue, command.OriginDeclared)
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
