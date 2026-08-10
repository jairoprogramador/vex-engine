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
//
// # Ya no lee del disco (spec 18 §5.2)
//
// Las declaraciones llegan cargadas por el resolutor de la cadena de pipeline.
// El handler leía `variables/<ambiente>/<paso>.yaml` justo antes de usarlo, que
// es la capa más interna decidiendo sobre material de la más externa —y la razón
// de que la identidad de la operación no se pudiera componer antes de ejecutar—.
// Lo que hace este handler es resolver, que es lo suyo.
//
// # Y es uno de los dos dueños de `parameter_resolved` (spec 19 §5.1)
//
// Los resolutores emiten en la CARGA del step y no cuando el valor llegue: aquí
// el par (declaración, valor resuelto) está junto y completo, que es exactamente
// lo que hace falta para que el hecho sea POR PARÁMETRO y no un digest agregado
// (N-3). El otro dueño es `05_vars_extractor_handler`, que emite lo que NACE al
// ejecutar; son dos momentos distintos del mismo hecho.
//
// Los LITERALES no lo emiten, y es una decisión: su declaración y su valor son
// la misma cosa, así que ya viajan enteros dentro del objeto de despliegue
// (spec 18) y un hecho por literal repetiría el material de identidad en cada
// corrida sin añadir nada. Lo que se registra es lo que el motor RESOLVIÓ.
type VarsHandler struct {
	StepBaseHandler
	loaded    *LoadedPipelinecode
	resolvers DeclarationResolvers
	facts     FactSink
}

var _ StepHandler = (*VarsHandler)(nil)

func NewVarsHandler(
	loaded *LoadedPipelinecode,
	resolvers DeclarationResolvers,
	facts FactSink) StepHandler {

	return &VarsHandler{
		StepBaseHandler: StepBaseHandler{Next: nil},
		loaded:          loaded,
		resolvers:       resolvers,
		facts:           facts,
	}
}

func (h *VarsHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	loaded, err := h.loaded.Get(request.StepFullName())
	if err != nil {
		return fmt.Errorf("cargar vars pipeline: %w", err)
	}
	declarations := loaded.DeclarationsCopy()

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

		// El `Origin` viaja como DATO y no como conclusión: `resolved` es lo que
		// permite distinguir «se lo dio una declaración» de «lo derivó el motor»
		// sin que el consumidor reconstruya el cableado. El valor no entra —entra
		// su resumen—, que es la regla de la spec 14 aplicada al registro.
		if err := h.facts.ParameterResolved(ctx, command.ParameterFact{
			Name:   declaration.Name(),
			Source: command.OriginResolved,
			Value:  value,
		}); err != nil {
			return fmt.Errorf("registrar la variable resuelta '%s': %w", declaration.Name(), err)
		}
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
