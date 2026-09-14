package cache

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/state"
)

// Entry es una entrada del ÍNDICE: un puntero de contenido a posición.
//
//	«¿este contenido exacto ya corrió alguna vez, y cuál fue el registro?»
//
// Hasta la spec 10 esto era una entrada de sola presencia con un TTL, y era LA
// respuesta a «¿hay que re-ejecutar?». Ya no: desde la spec 11 esa decisión lee
// el ÚLTIMO registro de la clave de estado, y este índice **no participa en
// ella**. Borrarlo entero no cambia una sola decisión del motor.
//
// Dos cosas se fueron con el cambio de papel, y las dos por el mismo motivo
// —eran estado propio de una estructura que tiene que ser derivable—:
//
//   - `ExpiresAt` y el TTL de 30 días. La expiración es una regla sobre el
//     registro (su edad), no sobre el índice; se declara en el pipelinecode con
//     la spec 15.
//   - `Provenance`. Vive en el registro, que es donde el hecho ocurrió.
//
// Lo que queda es exactamente lo que hace falta para reconstruirlo recorriendo
// los registros: a qué clave de estado y a qué registro apunta este contenido.
type Entry struct {
	StateKey state.Key
	RecordID state.RecordID
}

// NewEntry compone la entrada que apunta al registro que un step acaba de
// dejar.
//
// Un puntero incompleto es un ERROR y no una entrada degradada: apuntar a
// ninguna parte es peor que no apuntar, porque un índice con entradas rotas deja
// de ser reconstruible sin distinguir cuáles lo están.
func NewEntry(stateKey state.Key, recordID state.RecordID) (Entry, error) {
	if stateKey.IsZero() {
		return Entry{}, fmt.Errorf("cache: la entrada de índice no tiene clave de estado")
	}
	if recordID.IsZero() {
		return Entry{}, fmt.Errorf("cache: la entrada de índice no apunta a ningún registro")
	}
	return Entry{StateKey: stateKey, RecordID: recordID}, nil
}

// IsZero indica que no hay entrada.
func (e Entry) IsZero() bool {
	return e.StateKey.IsZero() || e.RecordID.IsZero()
}
