package step

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

type StepRunnerHandler struct {
	StepBaseHandler
	commandRepository PipelineCommandRepository
	policyBuilder     *status.PolicyBuilder
}

var _ StepHandler = (*StepRunnerHandler)(nil)

func NewStepRunnerHandler(
	commandRepository PipelineCommandRepository,
	policyBuilder *status.PolicyBuilder) StepHandler {

	return &StepRunnerHandler{
		StepBaseHandler:   StepBaseHandler{Next: nil},
		commandRepository: commandRepository,
		policyBuilder:     policyBuilder,
	}
}

func (h *StepRunnerHandler) Handle(ctx *context.Context, request *StepRequestHandler) error {
	commands, err := h.commandRepository.Get(ctx, request.PipelineLocalPath(), request.StepFullName())
	if err != nil {
		return fmt.Errorf("cargar commands: %w", err)
	}

	// Un step sin comandos no es un éxito: es un skip con razón. La diferencia
	// no es de vocabulario —el step deja de persistir estado de re-ejecución, que
	// es lo que hacía que un `commands.yaml` vacío quedara escrito como «sin
	// cambios» para siempre (spec 04 §5.3, D-A12).
	if len(commands) == 0 {
		request.MarkStepSkipped(SkipReasonNoCommands)
		request.Emit(fmt.Sprintf("%s se salta: no hay comandos para ejecutar (%s)",
			request.StepNameExe(), SkipReasonNoCommands))
		if h.Next != nil {
			return h.Next.Handle(ctx, request)
		}
		return nil
	}

	// Un step que el motor no sabe evaluar aborta la ejecución aquí (spec 05
	// §5.2). Antes `Build` no fallaba nunca: devolvía una policy vacía, y una
	// policy vacía se saltaba siempre. Este `return` es el que convierte aquel
	// silencio en un fallo con nombre.
	policy, err := h.policyBuilder.Build(request.StepName())
	if err != nil {
		return fmt.Errorf("construir policy: %w", err)
	}

	ctxRule := status.RuleContext{
		status.CurrentTimeParam:          request.StartedAt(),
		status.InstCurrentParam:          commands,
		status.VariablesCurrentParam:     request.AccumulatedVars(),
		status.ProjectStatusCurrentParam: request.ProjectStatus(),
		status.ProjectUrlParam:           request.ProjectUrl(),
		status.PipelineUrlParam:          request.PipelineUrl(),
		status.EnvironmentParam:          request.Environment(),
		status.StepParam:                 request.StepNameExe(),
	}

	// Evaluar ya no escribe nada: la policy responde y devuelve lo que observó
	// (spec 09 §5.1). Quien persiste esa observación es el camino de éxito de
	// `StepExecutable`, después del último comando.
	decision, evidences, err := policy.Evaluate(ctxRule)
	if err != nil {
		// ADVERTENCIA VISIBLE, no razón de negocio (spec 09 §5.3). Un fallo de
		// infraestructura no es «el código cambió», y hasta la spec 09 este error
		// se emitía con el mismo tono que cualquier otra línea y se seguía.
		//
		// La asimetría con el `return` de `Build`, veinte líneas más arriba, es
		// deliberada y hay que conservarla: `Build` falla cuando el motor no sabe
		// QUÉ comprobar —pipelinecode inválido, se aborta— y `Evaluate` cuando no
		// pudo AVERIGUAR si algo cambió —fallo de infraestructura, se ejecuta y se
		// dice—. Fundir las dos convertiría un pipelinecode inválido en un
		// despliegue que se ejecuta igual.
		request.Emit(fmt.Sprintf("advertencia: no se pudo determinar el estado de re-ejecución de %s: %v",
			request.StepNameExe(), err))
	}
	if decision.ShouldRun() {
		// La evidencia queda anotada ANTES de ejecutar, pero solo se escribe si
		// se llega al final. Anotar no persiste nada.
		request.RecordStatusEvidence(ctxRule, evidences)

		if decision.IsUndetermined() {
			request.Emit(fmt.Sprintf("Ejecutando %s sin poder determinar si cambió: %s",
				request.StepNameExe(), decision.Reason()))
		} else {
			request.Emit(fmt.Sprintf("Ejecutando %s: %s", request.StepNameExe(), decision.Reason()))
		}
		for _, command := range commands {
			request.AddCommand(command)
			if err := request.Execute(); err != nil {
				return err
			}
		}
		request.Emit(fmt.Sprintf("%s ejecutado correctamente", request.StepNameExe()))
	} else {
		request.Emit(fmt.Sprintf("%s ya fue ejecutado y se mantiene sin cambios", request.StepNameExe()))
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
