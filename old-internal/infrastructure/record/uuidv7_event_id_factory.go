// Package record es por donde salen los hechos del motor: la fábrica de
// identificadores y el sink que los escribe.
//
// El dominio (`internal/domain/record`) no escribe un byte y no conoce ni el
// reloj ni la aleatoriedad; las dos cosas entran por aquí.
package record

import (
	"crypto/rand"
	"fmt"
	"time"

	domRecord "github.com/jairoprogramador/vex-engine/old-internal/domain/record"
)

// eventIDEntropyLen son los bytes de aleatoriedad que consume un UUIDv7.
const eventIDEntropyLen = 10

var _ domRecord.EventIDFactory = (*UUIDv7EventIDFactory)(nil)

// UUIDv7EventIDFactory pone la aleatoriedad de cada `event_id`.
//
// Vive en infraestructura por la misma razón que el reloj, y tiene gemelo exacto
// en `infrastructure/state/ulid_record_id_factory.go`: `crypto/rand` en el
// dominio sería un `time.Now()` con otro nombre —una dependencia oculta que hace
// irreproducible lo que la usa—.
//
// **No se usa `uuid.NewV7()`**, aunque exista: esa función lleva dentro el reloj
// y el `crypto/rand`, así que el instante del evento no sería el que el `Clock`
// del motor dice. De la librería se usa el tipo y su formato, que es lo que la
// decisión de la spec 17 §5.5 defendía.
type UUIDv7EventIDFactory struct{}

func NewUUIDv7EventIDFactory() domRecord.EventIDFactory {
	return &UUIDv7EventIDFactory{}
}

func (f *UUIDv7EventIDFactory) New(at time.Time) (domRecord.EventID, error) {
	entropy := make([]byte, eventIDEntropyLen)
	if _, err := rand.Read(entropy); err != nil {
		return domRecord.EventID{}, fmt.Errorf("uuidv7 event id factory: entropía: %w", err)
	}
	return domRecord.NewEventID(at, entropy)
}
