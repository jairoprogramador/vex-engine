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

	if len(commands) == 0 {
		request.Emit("no hay comandos para ejecutar")
		return nil
	}

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

	decision, err := policy.Evaluate(ctxRule)
	if err != nil {
		request.Emit(fmt.Sprintf("error al evaluar policy: %v", err))
	}
	if decision.ShouldRun() {
		request.Emit(fmt.Sprintf("Ejecutando %s: %s", request.StepNameExe(), decision.Reason()))
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
