package pipeline

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/record"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/sync"
)

// PipelineRunnerHandler recorre los steps de la operación y, desde la spec 21,
// EMPUJA lo registrado al terminar cada uno.
//
// # Por qué el empuje va aquí y no dentro del ejecutable del step
//
// Porque lo que se empuja no es «lo de este step»: es todo lo pendiente del
// intento desde el `ack`, y eso incluye los hechos del clon viejo, los del
// resolutor y los de los steps anteriores que no llegaron. Colgarlo del step
// invitaría a mandar sólo su tramo, que es justo la selección que hace falta NO
// hacer para que un fallo se recupere solo.
//
// Y va después del step —no antes— porque lo que acota la pérdida es tener en el
// destino lo que ya ocurrió. Empujar antes de ejecutar sería empujar lo que ya se
// empujó.
type PipelineRunnerHandler struct {
	PipelineBaseHandler

	synchronizer *sync.Synchronizer
}

var _ PipelineHandler = (*PipelineRunnerHandler)(nil)

func NewPipelineRunnerHandler(synchronizer *sync.Synchronizer) PipelineHandler {
	return &PipelineRunnerHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
		synchronizer:        synchronizer,
	}
}

func (h *PipelineRunnerHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {

	request.Emit("Iniciando la ejecución")
	request.Emit(fmt.Sprintf("  - Entorno: %s", request.Environment()))
	request.Emit(fmt.Sprintf("  - Versión: %s", request.ProjectVersion()))
	request.Emit(fmt.Sprintf("  - Commit: %s", request.ProjectHeadHash()))

	// El enlace ocurre ANTES del primer step y no en el primer empuje: un intento
	// que falla dentro del step 1 no llega a ningún `Push` de este bucle, y el
	// cierre —que sí ocurre siempre— tiene que poder empujar su tira igual. Sin
	// esto, el caso de fallo sería justamente el que se pierde.
	h.synchronizer.Bind(ctx, record.EventStream{
		Deployment:  request.DeploymentID(),
		ExecutionID: request.ExecutionID(),
	}, request.Content(), deployment.ObjectMetadata{
		ProjectCommit:  request.ProjectHeadHash(),
		PipelineCommit: request.PipelineHeadHash(),
	})

	for _, stepName := range request.Steps() {
		request.SetStepName(stepName)
		err := request.Execute()

		// El empuje ocurre TAMBIÉN cuando el step falló, y antes de propagar el
		// error: el intento que falla es el que más interesa conservar, y sus
		// hechos ya están escritos en el área de trabajo. `Push` no devuelve error
		// —el registro nunca puede hacer fallar lo que observa— así que no hay
		// nada que componer con el del step.
		h.synchronizer.Push(ctx)

		if err != nil {
			return err
		}
	}

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}
