package record

import (
	"fmt"
	"strconv"
)

// Seq es la posición de un hecho dentro de su intento: 1, 2, 3…
//
// # Existe ADEMÁS del tiempo, y no en su lugar
//
// El reloj no es confiable —un contenedor, una máquina remota, un ajuste NTP—,
// y el orden sí importa para plegar. **`Fold` ordena por `seq`, nunca por
// `time`**, y hay un test que lo comprueba desordenando el slice por instante.
//
// El instante se conserva porque responde otra pregunta —«¿cuánto duró esto?»,
// «¿cuándo pasó?»— y porque un dato observado no se tira. Lo que no hace es
// decidir el orden.
type Seq struct {
	position uint64
}

// FirstSeq es la posición del primer hecho de un intento.
func FirstSeq() Seq { return Seq{position: 1} }

// NewSeq construye la posición. El cero no es una posición: es el valor cero del
// tipo y significa «no consta».
func NewSeq(position uint64) (Seq, error) {
	if position < 1 {
		return Seq{}, fmt.Errorf("record: %d no es una posición de evento", position)
	}
	return Seq{position: position}, nil
}

// Next es la posición siguiente.
func (s Seq) Next() Seq { return Seq{position: s.position + 1} }

// Position es el número de la posición.
func (s Seq) Position() uint64 { return s.position }

// String es la forma legible.
func (s Seq) String() string {
	if s.IsZero() {
		return ""
	}
	return strconv.FormatUint(s.position, 10)
}

// IsZero indica que no consta la posición.
func (s Seq) IsZero() bool { return s.position < 1 }

// Before es el orden de plegado.
func (s Seq) Before(other Seq) bool { return s.position < other.position }

// Equals es la regla de igualdad del value object.
func (s Seq) Equals(other Seq) bool { return s.position == other.position }
