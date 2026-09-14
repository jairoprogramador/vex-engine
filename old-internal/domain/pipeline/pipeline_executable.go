package pipeline

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
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
			executionContext.Emit("Pipeline iniciado")
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
		// Limpieza garantizada: corre falle o no la cadena (spec 06 §5.1). Y su
		// error se propaga en vez de descartarse: una plantilla que se queda
		// interpolada en el workdir hace que la ejecución siguiente ya no
		// encuentre los `${var.…}` que buscar, y el fallo de mañana es más
		// confuso que el de hoy.
		func() error {
			if err := executionContext.RestoreFileSessions(); err != nil {
				return fmt.Errorf("restaurar las plantillas del pipeline: %w", err)
			}
			return nil
		},
		// Sin cierre de ciclo: el par del INTENTO no cuelga de aquí sino de
		// `CreateExecutionUseCase`, la única capa que ve tanto el éxito como el
		// fallo (spec 19 §5.1). Esta cadena no ve la cancelación —le llega como un
		// error cualquiera— y `attempt_finished` tiene que poder distinguirla.
		nil,
	)
}
