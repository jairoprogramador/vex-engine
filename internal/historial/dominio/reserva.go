package dominio

import (
	"slices"
	"time"
)

// Reserva: desde cuándo el dueño del negocio decide él los lanzamientos de un ambiente, o cuándo lo liberó.
type Reserva struct {
	Reservado bool // falso: una liberación
	Instante  time.Time
}

// ReservasDeUnAmbiente son las reservas y liberaciones de un ambiente, en orden. No protegen ninguna
// invariante más allá de su propio registro.
type ReservasDeUnAmbiente struct {
	ambiente Ambiente
	reservas []Reserva
	leidos   int
}

// ReconstituirReservas rehace las reservas leídas del almacén.
func ReconstituirReservas(ambiente Ambiente, reservas []Reserva) *ReservasDeUnAmbiente {
	return &ReservasDeUnAmbiente{ambiente: ambiente, reservas: slices.Clone(reservas), leidos: len(reservas)}
}

func (r *ReservasDeUnAmbiente) Ambiente() Ambiente { return r.ambiente }
func (r *ReservasDeUnAmbiente) Leidos() int        { return r.leidos }

// Nuevas son las añadidas desde que se leyeron.
func (r *ReservasDeUnAmbiente) Nuevas() []Reserva { return slices.Clone(r.reservas[r.leidos:]) }

// Ultima dice si el ambiente está reservado ahora.
func (r *ReservasDeUnAmbiente) Ultima() (Reserva, bool) {
	if len(r.reservas) == 0 {
		return Reserva{}, false
	}
	return r.reservas[len(r.reservas)-1], true
}

// Reservar o liberar el ambiente.
func (r *ReservasDeUnAmbiente) Reservar(reservado bool, instante time.Time) error {
	if instante.IsZero() {
		return rechazo("una reserva necesita su instante")
	}
	r.reservas = append(r.reservas, Reserva{Reservado: reservado, Instante: instante})
	return nil
}
