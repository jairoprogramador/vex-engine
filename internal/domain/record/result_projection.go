package record

import (
	"context"
	"errors"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
)

// ErrAttemptNoConsta es la respuesta a preguntar por un intento del que no hay
// ni una tira.
//
// Es un centinela y no un `AttemptResult` vacío porque las dos cosas son
// distintas y la diferencia importa: `Fold(nil)` pliega a `interrupted` —un
// intento que empezó y del que no se escribió el desenlace— y esto es «aquí
// nunca hubo nada». Devolver lo primero ante lo segundo convertiría un
// identificador mal escrito en un diagnóstico de interrupción.
var ErrAttemptNoConsta = errors.New("record: no consta ningún hecho de ese intento")

// ResultProjection es el MODELO DE LECTURA del registro (spec 22 §5.1).
//
// # Es el otro lado de `EventSink`, y son dos puertos a propósito
//
// Los hechos son el modelo de escritura; `AttemptResult` es una proyección
// derivada. Que la lectura y la escritura tengan puertos distintos es la
// segregación de interfaces aplicada donde de verdad se cobra: **un lector no
// debe poder escribir** en una tienda cuya regla de vida es «permanente».
//
// # Y es lo que permite cambiar la estrategia de consulta sin tocar un emisor
//
// La v1 responde por ESCANEO —lee los JSONL y aplica `Fold`—, que es suficiente
// durante meses y es además la implementación de REFERENCIA contra la que se
// valida el pliegue del backend (spec 26). Cuando el escaneo se quede corto,
// detrás de este puerto entra un índice (`index.db`, spec 22 §5.4) sin que nada
// aguas arriba lo note: es una vista materializada de los eventos, reconstruible
// por definición, así que añadirla no migra nada.
//
// # Cero lógica de plegado propia
//
// Las dos operaciones terminan en `Fold`, que ya existe y es puro. Si una
// implementación de este puerto necesitara derivar algo que `Fold` no deriva, lo
// que estaría mal es `Fold` — y detectarlo a tiempo es la razón de que esta
// superficie exista mientras todavía hay pocos datos permanentes emitidos.
type ResultProjection interface {
	// Attempt reconstruye UN intento de UN despliegue.
	//
	// `attempt` es el value object y no un entero, que es el ajuste de firma que
	// la spec 17 §9 pidió al implementar: un `attempt: 0` es «no consta», y una
	// consulta sin sujeto tiene que fallar en el borde en vez de convertirse en
	// una búsqueda que devuelve cualquier cosa.
	//
	// Ausencia total de hechos es `ErrAttemptNoConsta`. Una tira INCOMPLETA no lo
	// es: pliega a `interrupted` con el último step alcanzado, y ésa es la
	// respuesta correcta —es la propiedad que justificó el event sourcing—.
	Attempt(
		ctx *context.Context,
		id deployment.DeploymentID,
		attempt deployment.Attempt) (AttemptResult, error)

	// History son los intentos registrados contra un ambiente, del más reciente
	// al más antiguo.
	//
	// El sujeto es el `Destination` —el ambiente al que se despliega, no el sitio
	// al que se empuja el registro (spec 17 §5.7)—. Existe porque nombrar un
	// destino de rollback («vuelve a la ejecución X», spec 28) exige poder listar
	// los intentos de un ambiente; de ahí que el consumidor necesite además saber
	// cuáles son destinos VÁLIDOS, que es lo que `AttemptResult.IsValidTarget`
	// responde.
	//
	// `limit <= 0` significa «todos». Una lista vacía NO es un error: un ambiente
	// sin historia es un ambiente al que no se ha desplegado.
	History(
		ctx *context.Context,
		destination deployment.Destination,
		limit int) ([]AttemptResult, error)
}
