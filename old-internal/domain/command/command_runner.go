package command

import "context"

type CommandRunner interface {
	Run(ctx *context.Context, command string, workDir string) (CommandResult, error)
}
