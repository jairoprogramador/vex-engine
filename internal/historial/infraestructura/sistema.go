package infraestructura

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

// RelojDelSistema da la hora de la máquina, en UTC.
type RelojDelSistema struct{}

var _ dominio.Reloj = RelojDelSistema{}

func (RelojDelSistema) Ahora() time.Time { return time.Now().UTC() }

// IdentidadesUUID da identidades UUIDv7: únicas sin coordinarse, y ordenadas por el instante en que nacen.
type IdentidadesUUID struct{}

var _ dominio.Identidades = IdentidadesUUID{}

func (IdentidadesUUID) NuevoIntento() (dominio.IdIntento, error) {
	id, err := nuevoUUID()
	return dominio.IdIntento(id), err
}

func (IdentidadesUUID) NuevoDespliegue() (dominio.IdDespliegue, error) {
	id, err := nuevoUUID()
	return dominio.IdDespliegue(id), err
}

func (IdentidadesUUID) NuevoLanzamiento() (dominio.IdLanzamiento, error) {
	id, err := nuevoUUID()
	return dominio.IdLanzamiento(id), err
}

func nuevoUUID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("identidades: %w", err)
	}
	return id.String(), nil
}
