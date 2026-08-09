package record

import (
	"fmt"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
)

// Event es un DOMAIN EVENT en sentido estricto: un hecho en pasado, inmutable,
// que nunca se corrige. Una corrección es otro evento.
//
// El sobre lleva lo que TODO hecho tiene —identidad, posición, instante e
// intento— y la carga útil lleva lo propio de cada tipo. Que `attempt` viva en
// el sobre y no en la carga es la decisión I-2: es un dato derivado de un conteo,
// y su sitio es el evento, no el nombre del archivo.
//
// El `deployment_id` NO está en el sobre: viaja en `attempt_started` y en la
// ruta del archivo (`events/<deployment_id>/<execution_id>.jsonl`). Repetirlo en
// cada hecho sería guardar el mismo dato tres veces y dejar que las tres copias
// se contradigan.
type Event struct {
	id      EventID
	seq     Seq
	at      time.Time
	attempt deployment.Attempt
	payload Payload
}

// Payload es lo propio de cada tipo de hecho.
//
// Las cargas son structs con campos exportados y no value objects con
// constructor, por lo mismo que `cache.Material`: son material explícito y
// exhaustivo, y lo que se quiere es que añadir un campo sea un cambio de tipo
// visible en el compilador. La invariante de cada una vive en su `Validate`, que
// `NewEvent` llama siempre.
type Payload interface {
	// Type es el tipo de hecho que esta carga representa.
	Type() EventType

	// Validate comprueba la invariante propia del tipo.
	Validate() error
}

// NewEvent compone el hecho y valida el sobre y la carga.
//
// Un evento inválido es un ERROR en el emisor, no un hecho degradado que se
// escribe igual: el registro es la fuente de la que se deriva todo lo demás
// (spec 19), y un hecho a medias contamina el pliegue de todos los que vengan
// detrás.
func NewEvent(
	id EventID,
	seq Seq,
	at time.Time,
	attempt deployment.Attempt,
	payload Payload) (Event, error) {

	if id.IsZero() {
		return Event{}, fmt.Errorf("record: un evento sin event_id no es identificable")
	}
	if seq.IsZero() {
		return Event{}, fmt.Errorf("record: un evento sin seq no es plegable")
	}
	if at.IsZero() {
		return Event{}, fmt.Errorf("record: un evento sin instante no es fechable")
	}
	if attempt.IsZero() {
		return Event{}, fmt.Errorf("record: un evento sin intento no es atribuible")
	}
	if payload == nil {
		return Event{}, fmt.Errorf("record: un evento sin carga no dice qué pasó")
	}
	if err := payload.Validate(); err != nil {
		return Event{}, err
	}
	return Event{id: id, seq: seq, at: at, attempt: attempt, payload: payload}, nil
}

func (e Event) ID() EventID                 { return e.id }
func (e Event) Seq() Seq                    { return e.seq }
func (e Event) At() time.Time               { return e.at }
func (e Event) Attempt() deployment.Attempt { return e.attempt }
func (e Event) Payload() Payload            { return e.payload }

// Type es el tipo del hecho, o la cadena vacía si el evento es el valor cero.
func (e Event) Type() EventType {
	if e.payload == nil {
		return ""
	}
	return e.payload.Type()
}

// IsZero indica que no hay hecho.
func (e Event) IsZero() bool { return e.payload == nil || e.id.IsZero() }
