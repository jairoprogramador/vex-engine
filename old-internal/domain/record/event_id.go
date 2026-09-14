package record

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// eventIDEntropyLen son los 10 bytes de aleatoriedad que consume un UUIDv7. De
// ellos se usan 74 bits —12 en `rand_a` y 62 en `rand_b`—; los 6 restantes se
// descartan al colocar la versión y la variante.
const eventIDEntropyLen = 10

// EventID identifica un hecho. Es un **UUIDv7**, y la decisión se re-abrió una
// vez con base verificable (spec 17 §5.5).
//
// Los documentos previos la habían cerrado en ULID y anotaron el coste: una
// dependencia nueva. Pero `github.com/google/uuid` YA es dependencia directa
// (`go.mod`), y UUIDv7 ofrece exactamente las mismas garantías —ordenable por
// tiempo, único por construcción—, así que ULID añadiría una dependencia sin
// aportar nada. Si algún día se confirmara ULID, la spec se ajusta y no se
// vuelve a plantear.
//
// # Es circunstancial, y por eso NO es un hash de su contenido
//
// Mismo argumento que `state.RecordID`: dos hechos idénticos en instantes
// distintos son DOS hechos, y con un hash colapsarían en uno. La identidad de un
// evento no debe depender de lo que dice, sino de cuándo ocurrió.
//
// # El instante y la entropía llegan como datos
//
// Igual que en `state.NewRecordID`, y por la misma regla: `time.Now()` está
// prohibido en el dominio (spec 07) y `crypto/rand` sería un `time.Now()` con
// otro nombre. `uuid.NewV7()` hace las dos cosas por dentro, así que **no se
// usa**: lo que se usa de la librería es el tipo y su formato. Es lo que permite
// fijar un `event_id` entero en un test.
type EventID struct {
	value uuid.UUID
}

// NewEventID compone el identificador según el formato de UUIDv7 (RFC 9562):
//
//	48 bits  unix_ts_ms
//	 4 bits  versión (0111)
//	12 bits  rand_a
//	 2 bits  variante (10)
//	62 bits  rand_b
func NewEventID(at time.Time, entropy []byte) (EventID, error) {
	if at.IsZero() {
		return EventID{}, fmt.Errorf("record: un event_id sin instante no es ordenable")
	}
	if len(entropy) != eventIDEntropyLen {
		return EventID{}, fmt.Errorf(
			"record: la entropía del event_id son %d bytes, no %d",
			eventIDEntropyLen, len(entropy))
	}

	milis := at.UTC().UnixMilli()
	if milis < 0 || milis >= 1<<48 {
		return EventID{}, fmt.Errorf(
			"record: el instante %s no cabe en los 48 bits de un UUIDv7",
			at.UTC().Format(time.RFC3339))
	}

	var bytes [16]byte
	for i := 0; i < 6; i++ {
		bytes[i] = byte(milis >> (40 - 8*i))
	}
	bytes[6] = 0x70 | (entropy[0] & 0x0f)
	bytes[7] = entropy[1]
	bytes[8] = 0x80 | (entropy[2] & 0x3f)
	copy(bytes[9:], entropy[3:])

	value, err := uuid.FromBytes(bytes[:])
	if err != nil {
		return EventID{}, fmt.Errorf("record: componer el event_id: %w", err)
	}
	return EventID{value: value}, nil
}

// ParseEventID lee la forma canónica de un UUID. La necesita quien relee un
// archivo de eventos —el plegado del backend, la consulta de la spec 22—.
func ParseEventID(text string) (EventID, error) {
	if text == "" {
		return EventID{}, fmt.Errorf("record: event_id vacío")
	}
	value, err := uuid.Parse(text)
	if err != nil {
		return EventID{}, fmt.Errorf("record: %q no es un event_id: %w", text, err)
	}
	return EventID{value: value}, nil
}

// String es la forma canónica y externa: el UUID con guiones.
func (id EventID) String() string {
	if id.IsZero() {
		return ""
	}
	return id.value.String()
}

// IsZero indica que no hay identificador.
func (id EventID) IsZero() bool { return id.value == uuid.Nil }

// Equals es la regla de igualdad del value object.
func (id EventID) Equals(other EventID) bool { return id.value == other.value }

// EventIDFactory produce identificadores nuevos.
//
// Es un puerto y no una función por lo mismo que `state.RecordIDFactory`: la
// aleatoriedad es infraestructura, exactamente igual que el reloj. Lo implementa
// la spec 19, que es la que emite; aquí sólo se declara qué hace falta.
type EventIDFactory interface {
	New(at time.Time) (EventID, error)
}
