package command

import (
	"context"
	"fmt"
)

// VarsExtractorHandler es donde una variable NACE de la ejecución, y por eso es
// uno de los dos dueños de `parameter_resolved` (spec 19 §5.1).
//
// Los otros —los resolutores de declaraciones— emiten en la CARGA del step,
// antes del primer comando, porque ahí el par (declaración, valor resuelto) está
// junto y completo. Éste emite lo que NACE al ejecutar, que es la respuesta
// exacta a «¿qué dejó este step?» y ya no se confunde con nada desde la spec 14.
//
// La pregunta pasó a depender enteramente de este hecho por un camino que
// conviene tener escrito: un step que ejecuta y no produce nada escribe hoy un
// registro VACÍO, así que el registro dejó de servir para enumerar con qué
// valores corrió.
type VarsExtractorHandler struct {
	CommandBaseHandler
	facts FactSink
}

var _ CommandHandler = (*VarsExtractorHandler)(nil)

func NewVarsExtractorHandler(facts FactSink) CommandHandler {
	return &VarsExtractorHandler{
		CommandBaseHandler: CommandBaseHandler{Next: nil},
		facts:              facts,
	}
}

func (h *VarsExtractorHandler) Handle(ctx *context.Context, request *CommandRequestHandler) error {
	vars, err := ExtractVars(request.CommandNormalizedStdout(), request.CommandOutputs())
	if err != nil {
		return fmt.Errorf("extraer variables de output: %w", err)
	}

	// Aquí se leía `request.CommandWorkdirIsShared()` para marcar cada variable
	// con el ámbito deducido del primer segmento del workdir. Lo retira la
	// spec 13 §5.6: el ámbito es del STEP, así que todas las variables que este
	// comando produce ya lo tienen y la marca por variable sobraba.
	for name, value := range vars {
		commandVariable, err := NewCommandVariable(name, value)
		if err != nil {
			return fmt.Errorf("crear variable de comando: %w", err)
		}
		request.AddCommandVar(commandVariable)

		// `OriginRuntime`: lo que el mundo real devolvió al ejecutar. Es la
		// precedencia más alta, y por eso un literal declarado con el mismo nombre
		// pasa a ser lo que dice ser, un valor por defecto (spec 12 §5.1).
		executionVariable, err := NewVariable(name, value, OriginRuntime)
		if err != nil {
			return fmt.Errorf("crear variable de ejecución: %w", err)
		}
		request.AddAccumulatedVars(executionVariable)

		// Y se anota como PRODUCIDA por este step. Son dos destinos porque son dos
		// preguntas distintas: el mapa acumulado responde «¿qué ve el step
		// siguiente?» y esto responde «¿qué dejó éste?», que es lo único que su
		// registro debe guardar (spec 14 §6). Hasta ahora el registro guardaba el
		// mapa acumulado entero, así que un literal declarado volvía del almacén
		// como `OriginState` en la corrida siguiente y editarlo dejaba de surtir
		// efecto.
		request.AddProducedVar(executionVariable)

		// Y el hecho, UNO POR PARÁMETRO (N-3, la variante agregada se retira). El
		// `Origin` viaja como DATO y no como conclusión: el consumidor deriva de él
		// quién ganó sin reconstruir el cableado. El valor no entra —entra su
		// resumen—, que es la regla de la spec 14 aplicada al registro.
		if err := h.facts.ParameterResolved(ctx, ParameterFact{
			Name:   name,
			Source: OriginRuntime,
			Value:  value,
		}); err != nil {
			return fmt.Errorf("registrar la variable extraída '%s': %w", name, err)
		}
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
