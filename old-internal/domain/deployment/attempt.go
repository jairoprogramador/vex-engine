package deployment

import (
	"fmt"
	"strconv"
)

// Attempt es el número de intento de un mismo despliegue: la primera vez que se
// ejecuta `deployment_id` es el intento 1, un reintento es el 2.
//
// # No nombra el archivo de eventos, y ésa es la decisión I-2
//
// `events/<attempt>.jsonl` reintroducía el fallo que se acababa de quitar: dos
// intentos de dos despliegues distintos con el mismo número escriben el mismo
// archivo. Y `attempt` se deriva de un CONTEO —lectura seguida de escritura, sin
// atomicidad—, así que dos ejecuciones concurrentes obtienen el mismo número.
//
// El archivo lo nombra `execution_id`, que ya existe, ya es un UUID y es único
// por construcción; `attempt` pasa a ser un CAMPO del evento, que es donde debe
// estar un dato derivado.
type Attempt struct {
	number int
}

// FirstAttempt es el primer intento de un despliegue.
func FirstAttempt() Attempt { return Attempt{number: 1} }

// NewAttempt construye el intento a partir de su número.
//
// El cero no es un intento: es el valor cero del tipo y significa «no consta».
// Un intento negativo tampoco, y no por simetría — un conteo mal derivado tiene
// que fallar donde se compone, no producir un hecho que dice algo imposible.
func NewAttempt(number int) (Attempt, error) {
	if number < 1 {
		return Attempt{}, fmt.Errorf("deployment: %d no es un número de intento", number)
	}
	return Attempt{number: number}, nil
}

// Number es el número del intento.
func (a Attempt) Number() int { return a.number }

// Next es el intento siguiente.
func (a Attempt) Next() Attempt { return Attempt{number: a.number + 1} }

// String es la forma legible del intento.
func (a Attempt) String() string {
	if a.IsZero() {
		return ""
	}
	return strconv.Itoa(a.number)
}

// IsZero indica que no consta el intento.
func (a Attempt) IsZero() bool { return a.number < 1 }

// Equals es la regla de igualdad del value object.
func (a Attempt) Equals(other Attempt) bool { return a.number == other.number }
