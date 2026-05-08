package pipeline

import (
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type PipelineExecutable struct {
	command.BaseExecutable
	handler PipelineHandler
}

var _ command.Executable = (*PipelineExecutable)(nil)

func NewPipelineExecutable(handler PipelineHandler) *PipelineExecutable {
	return &PipelineExecutable{
		handler: handler,
	}
}

func (s *PipelineExecutable) Execute(executionContext *command.ExecutionContext) error {
	return s.Run(
		executionContext,
		func() error {
			executionContext.ResetFileSessions()
			return nil
		},
		func() error {
			request := NewPipelineRequestHandler(executionContext)
			err := s.handler.Handle(request.Ctx(), request)
			if err != nil {
				executionContext.Emit("Pipeline ejecucion fallida")
				executionContext.Emit(err.Error())
			} else {
				executionContext.Emit("Pipeline ejecutado correctamente")
			}
			return err
		},
		func() error {
			executionContext.RestoreFileSessions()
			return nil
		},
	)
}
