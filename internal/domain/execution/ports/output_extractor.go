package ports

import "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"

type OutputExtractor interface {
	ExtractVars(commandOutput string, outputs []vos.CommandOutput) (vos.VariableSet, error)
}
