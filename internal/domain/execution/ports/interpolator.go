package ports

import "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"

type Interpolator interface {
	Interpolate(input string, vars vos.VariableSet) (string, error)
}
