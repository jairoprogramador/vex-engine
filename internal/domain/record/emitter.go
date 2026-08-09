package record

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

// Emitter numera los hechos de un intento y los entrega al sink.
//
// Es un servicio de dominio: pone el sobre —`event_id`, `seq`, instante e
// intento— alrededor de la carga que le da quien PRESENCIA el hecho. Quien
// observa no tiene que saber en qué posición va su hecho ni de dónde sale un
// UUIDv7, y esa es la frontera que hace que «un solo dueño por hecho» (spec 19
// §5.1) sea sostenible.
//
// # El búfer, y por qué no es una optimización
//
// La tira de hechos vive en `events/<deployment_id>/<execution_id>.jsonl`, así
// que no hay dónde escribir hasta que el resolutor deriva el `deployment_id`
// (spec 18 §5.1). Pero hay un hecho que ocurre ANTES: `stale_clone_used`, que lo
// observa el clonador en la posición 02 —y tiene que observarlo él, porque es
// quien elige la fuente—.
//
// El emisor lo resuelve reteniendo lo que llega antes de `Open` y escribiéndolo
// al abrirse. Dos consecuencias que conviene tener escritas:
//
//   - **`seq` es el orden de OBSERVACIÓN, no el de escritura.** El clon viejo se
//     usó antes de que el intento tuviera identidad, y el pliegue tiene que poder
//     decirlo.
//   - **Lo retenido se pierde si la ejecución falla antes del resolutor.** Es
//     correcto y no un agujero: sin `deployment_id` no hay tira a la que
//     pertenezcan, y escribirlos en otro sitio sería inventar una segunda
//     dirección para los mismos hechos.
type Emitter struct {
	clock shared.Clock
	ids   EventIDFactory
	sink  EventSink

	// El motor es de un solo hilo, pero `seq` es lo único que da orden al
	// pliegue: que su corrección no dependa de que nadie añada una goroutine
	// cuesta un mutex.
	mu      sync.Mutex
	last    Seq
	attempt deployment.Attempt
	stream  EventStream
	opened  bool
	pending []observedFact
}

// observedFact es un hecho al que todavía le falta el sobre: se conoce su
// posición y su instante, no el intento al que pertenece.
type observedFact struct {
	seq     Seq
	at      time.Time
	payload Payload
}

func NewEmitter(clock shared.Clock, ids EventIDFactory, sink EventSink) *Emitter {
	return &Emitter{clock: clock, ids: ids, sink: sink}
}

// Emit registra un hecho. Si la tira todavía no está abierta lo retiene; si lo
// está, lo escribe.
//
// El instante sale del reloj inyectable, como todo instante del dominio
// (spec 07): `time.Now()` aquí haría irreproducible cualquier prueba de lo que
// se construya encima.
func (e *Emitter) Emit(ctx *context.Context, payload Payload) error {
	if payload == nil {
		return fmt.Errorf("record: no se puede emitir un hecho sin carga")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	fact := observedFact{seq: e.next(), at: e.clock.Now(), payload: payload}
	if !e.opened {
		e.pending = append(e.pending, fact)
		return nil
	}

	event, err := e.envelope(fact)
	if err != nil {
		return err
	}
	return e.sink.Append(ctx, e.stream, []Event{event})
}

// Open dice a qué tira pertenecen los hechos de este intento y escribe los que
// esperaban.
//
// Abrir dos veces es un error del emisor y no una reapertura: significaría que
// dos despliegues se están escribiendo en la misma numeración de `seq`.
func (e *Emitter) Open(ctx *context.Context, stream EventStream, attempt deployment.Attempt) error {
	if err := stream.Validate(); err != nil {
		return err
	}
	if attempt.IsZero() {
		return fmt.Errorf("record: la tira de %s no dice de qué intento es", stream.ExecutionID)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.opened {
		return fmt.Errorf("record: la tira de hechos ya estaba abierta")
	}
	e.stream = stream
	e.attempt = attempt
	e.opened = true

	if len(e.pending) == 0 {
		return nil
	}

	events := make([]Event, 0, len(e.pending))
	for _, fact := range e.pending {
		event, err := e.envelope(fact)
		if err != nil {
			return err
		}
		events = append(events, event)
	}
	e.pending = nil

	return e.sink.Append(ctx, e.stream, events)
}

// next avanza el contador de posiciones. El primero es 1.
func (e *Emitter) next() Seq {
	if e.last.IsZero() {
		e.last = FirstSeq()
		return e.last
	}
	e.last = e.last.Next()
	return e.last
}

// envelope compone el evento. Un hecho inválido es un error DEL EMISOR y no un
// evento degradado que se escribe igual: el registro es la fuente de la que se
// deriva todo lo demás, y un hecho roto contamina el pliegue de los que vengan
// detrás.
func (e *Emitter) envelope(fact observedFact) (Event, error) {
	id, err := e.ids.New(fact.at)
	if err != nil {
		return Event{}, fmt.Errorf("record: componer el identificador del hecho: %w", err)
	}
	return NewEvent(id, fact.seq, fact.at, e.attempt, fact.payload)
}
