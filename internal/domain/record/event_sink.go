package record

import (
	"context"
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
)

// EventStream es la tira de hechos de UN intento: a qué despliegue pertenece y
// qué ejecución la escribió.
//
// Son los dos datos que el layout de la spec 17 §5.8 pone en la ruta
// (`events/<deployment_id>/<execution_id>.jsonl`) y por eso viajan juntos y
// fuera del sobre de cada evento: repetirlos en cada hecho sería guardar el
// mismo dato tantas veces como hechos haya y dejar que las copias se
// contradigan.
//
// El archivo lo nombra `execution_id` y no `attempt` (decisión I-2): dos intentos
// de dos despliegues distintos con el mismo número escribirían el mismo archivo,
// y `attempt` se deriva de un conteo sin atomicidad.
type EventStream struct {
	Deployment  deployment.DeploymentID
	ExecutionID string
}

// Validate exige los dos: una tira sin despliegue no tiene dónde vivir, y una
// sin ejecución no se distingue de la de al lado.
func (s EventStream) Validate() error {
	if s.Deployment.IsZero() {
		return fmt.Errorf("record: la tira de hechos no dice de qué despliegue es")
	}
	if s.ExecutionID == "" {
		return fmt.Errorf("record: la tira de hechos no dice qué ejecución la escribió")
	}
	return nil
}

// IsZero indica que no hay tira.
func (s EventStream) IsZero() bool { return s.Deployment.IsZero() && s.ExecutionID == "" }

// EventSink es por donde salen los hechos.
//
// Anexa en bloque y no de uno en uno porque el emisor tiene un caso en el que
// escribe varios de golpe: los que ocurrieron ANTES de conocerse el
// `deployment_id` y esperaban a saber a qué tira pertenecen (spec 18 §5.4). Un
// puerto de a uno obligaría a que el emisor hiciera N llamadas donde el
// adaptador puede abrir el archivo una vez.
//
// La escritura es SÍNCRONA por diseño y no hay bus: un solo proceso, un solo
// consumidor, y el orden lo garantiza `seq`. Asincronía aquí significaría perder
// los hechos justo en el caso que justifica el modelo entero —la máquina que
// muere a mitad—.
type EventSink interface {
	Append(ctx *context.Context, stream EventStream, events []Event) error
}
