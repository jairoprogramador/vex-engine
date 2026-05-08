package command

import "strings"

type CommandResult struct {
	rawStdout        string // Salida estándar original y sin procesar.
	rawStderr        string // Salida de error original y sin procesar.
	normalizedStdout string // Salida estándar normalizada (sin ANSI, trim, etc.).
	normalizedStderr string // Salida de error normalizada.
	exitCode         int    // Código de salida del comando.
	err              error
}

func NewCommandResult(rawStdout, rawStderr, normalizedStdout, normalizedStderr string) CommandResult {
	return CommandResult{
		rawStdout:        rawStdout,
		rawStderr:        rawStderr,
		normalizedStdout: normalizedStdout,
		normalizedStderr: normalizedStderr,
	}
}

func (cr CommandResult) CombinedOutput() string {
	var builder strings.Builder
	builder.WriteString(cr.rawStdout)
	builder.WriteString(cr.rawStderr)
	return builder.String()
}

func (cr CommandResult) NormalizedStdout() string {
	return cr.normalizedStdout
}

func (cr CommandResult) NormalizedStderr() string {
	return cr.normalizedStderr
}

func (cr CommandResult) ExitCode() int {
	return cr.exitCode
}

func (cr CommandResult) SetExitCode(exitCode int) {
	cr.exitCode = exitCode
}

func (cr CommandResult) Error() error {
	return cr.err
}

func (cr CommandResult) SetError(error error) {
	cr.err = error
}
