package state

import (
	"crypto/rand"
	"fmt"
	"time"

	domState "github.com/jairoprogramador/vex-engine/old-internal/domain/state"
)

var _ domState.RecordIDFactory = (*ULIDRecordIDFactory)(nil)

// ULIDRecordIDFactory pone los 80 bits de aleatoriedad de cada `record_id`.
//
// Vive en infraestructura por la misma razón que el reloj: `crypto/rand` en el
// dominio sería un `time.Now()` con otro nombre —una dependencia oculta que
// hace irreproducible lo que la usa—. Con el puerto, un test fija el ULID
// entero.
type ULIDRecordIDFactory struct{}

func NewULIDRecordIDFactory() domState.RecordIDFactory {
	return &ULIDRecordIDFactory{}
}

func (f *ULIDRecordIDFactory) New(at time.Time) (domState.RecordID, error) {
	entropy := make([]byte, 10)
	if _, err := rand.Read(entropy); err != nil {
		return domState.RecordID{}, fmt.Errorf("ulid record id factory: entropía: %w", err)
	}
	return domState.NewRecordID(at, entropy)
}
