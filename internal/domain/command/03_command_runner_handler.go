package command

import (
	"context"
	"fmt"
	"path/filepath"
)

type CommandRunnerHandler struct {
	CommandBaseHandler
	runner CommandRunner
}

var _ CommandHandler = (*CommandRunnerHandler)(nil)

func NewCommandRunnerHandler(runner CommandRunner) CommandHandler {
	return &CommandRunnerHandler{
		CommandBaseHandler: CommandBaseHandler{Next: nil},
		runner:             runner,
	}
}

func (h *CommandRunnerHandler) Handle(ctx *context.Context, request *CommandRequestHandler) error {
	request.Emit(fmt.Sprintf("Comando: %s", request.CommandName()))

	localStepWorkdirPath, ok := request.LocalStepWorkdirPath()

	if !ok {
		return fmt.Errorf("variable de step workdir no encontrada")
	}

	commandWorkdir := request.CommandWorkdir()

	var runWorkdir string
	if commandWorkdir == "" || commandWorkdir == "." {
		runWorkdir = request.ProjectLocalPath()
	} else {
		runWorkdir = filepath.Join(localStepWorkdirPath.Value(), commandWorkdir)
	}

	result, err := h.runner.Run(ctx, request.CommandInterpolatedCmd(), runWorkdir)
	if err != nil {
		return fmt.Errorf("error en la ejecución del comando '%s': %w, %s", request.CommandName(), err, result.CombinedOutput())
	}
	if result.ExitCode() != 0 {
		return fmt.Errorf("comando '%s' falló con exit code %d:\n%s",
			request.CommandName(), result.ExitCode(), result.CombinedOutput())
	}

	request.SetCommandResult(result)

	if request.CommandShow() {
		if out := request.CommandNormalizedStdout(); out != "" {
			request.Emit(fmt.Sprintf("[%s] %s", request.CommandName(), out))
		}
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
