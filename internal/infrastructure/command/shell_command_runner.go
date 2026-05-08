package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

type ShellCommandRunner struct{}

func NewShellCommandRunner() command.CommandRunner {
	return &ShellCommandRunner{}
}

func (r ShellCommandRunner) Run(ctx *context.Context, commandParam string, workDir string) (command.CommandResult, error) {
	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(*ctx, "cmd", "/C", commandParam)
	} else {
		cmd = exec.CommandContext(*ctx, "sh", "-c", commandParam)
	}

	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := command.NewCommandResult(
		stdout.String(),
		stderr.String(),
		strings.TrimSpace(ansiRegex.ReplaceAllString(strings.ReplaceAll(stdout.String(), "\r\n", "\n"), "")),
		strings.TrimSpace(ansiRegex.ReplaceAllString(strings.ReplaceAll(stderr.String(), "\r\n", "\n"), "")),
	)

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.SetExitCode(exitErr.ExitCode())
			return result, nil
		}
		return command.NewCommandResult("", "", "", ""), fmt.Errorf("error al intentar ejecutar el comando '%s': %w", commandParam, err)
	}

	result.SetExitCode(0)
	return result, nil
}
