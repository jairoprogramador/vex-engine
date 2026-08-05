package command

import "fmt"

// DefaultFailureExitCode es el exit code que se le atribuye a una ejecución
// que falló por algo que no fue un comando (un clone, una validación del
// pipelinecode, una plantilla que no se pudo restaurar).
const DefaultFailureExitCode = 1

// CommandFailedError es el fallo de un comando del pipelinecode CON su exit
// code intacto. Antes el número se formateaba dentro del mensaje y se perdía
// ahí: el agregado no tenía de dónde sacar el `exit_code` de la ejecución
// (spec 07 §7).
type CommandFailedError struct {
	CommandName string
	ExitCodeVal int
	Output      string
}

func NewCommandFailedError(commandName string, exitCode int, output string) *CommandFailedError {
	return &CommandFailedError{
		CommandName: commandName,
		ExitCodeVal: exitCode,
		Output:      output,
	}
}

func (e *CommandFailedError) Error() string {
	return fmt.Sprintf("comando '%s' falló con exit code %d:\n%s",
		e.CommandName, e.ExitCodeVal, e.Output)
}

func (e *CommandFailedError) ExitCode() int {
	return e.ExitCodeVal
}
