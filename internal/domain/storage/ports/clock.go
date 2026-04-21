package ports

import "time"

// Clock abstrae time.Now() para hacer testeable cualquier lógica basada en el tiempo.
type Clock interface {
	Now() time.Time
}
