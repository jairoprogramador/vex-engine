package ports

import "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"

type FileProcessor interface {
	Process(absPathsFiles []string, vars vos.VariableSet) error
	Restore() error
}
