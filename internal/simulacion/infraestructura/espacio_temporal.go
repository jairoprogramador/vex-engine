package infraestructura

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/jairoprogramador/vex-engine/internal/simulacion/dominio"
)

// EspacioTemporal da un id de invocación nuevo por cada ambiente que se recorre (UUIDv7: único sin
// coordinarse) — mismo patrón que historial/infraestructura.IdentidadesUUID.
type EspacioTemporal struct{}

var _ dominio.EspacioTemporal = EspacioTemporal{}

func (EspacioTemporal) Nuevo() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("simulación: un id de simulación nuevo: %w", err)
	}
	return id.String(), nil
}
