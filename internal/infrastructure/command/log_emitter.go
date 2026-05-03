package execution

/*
import "github.com/jairoprogramador/vex-engine/internal/domain/command"

type FuncLogEmitter struct {
	emitFn func(executionID command.ExecutionID, line string)
}

func NewFuncLogEmitter(emitFn func(executionID command.ExecutionID, line string)) command.LogEmitter {
	if emitFn == nil {
		emitFn = func(_ command.ExecutionID, _ string) {}
	}
	return &FuncLogEmitter{emitFn: emitFn}
}

func (e *FuncLogEmitter) Emit(executionID command.ExecutionID, line string) {
	e.emitFn(executionID, line)
}

var _ command.LogEmitter = (*FuncLogEmitter)(nil)
*/
