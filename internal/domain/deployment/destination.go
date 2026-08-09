package deployment

import (
	"fmt"
	"strings"
)

// DestinationReserved es la palabra que un ambiente no puede llamarse.
//
// `shared` está reservada desde la spec 04 §5.4 y la reserva sobrevive al
// rediseño del ámbito: un ambiente con ese nombre haría ambiguo el ámbito
// compartido en toda dirección que lo transporte.
const DestinationReserved = "shared"

// Destination es el ambiente al que se despliega, y es el nombre que se resolvió
// —el `value` de `environments.yaml`, no su `name`—.
//
// Se llama `Destination` y no `Target` por la colisión que la spec 17 §5.7
// cierra: `sync.Sink` es el sitio al que se EMPUJA el registro, y este es el
// ambiente al que se DESPLIEGA. Dos conceptos que en inglés se llamaban igual y
// que no tienen nada que ver.
//
// El nombre se valida contra lo que puede romper una ruta, con las mismas reglas
// que `state.NewEnvironmentScope`: el ambiente termina siendo un tramo de la
// ruta del almacén, y un nombre que la rompa no debe producir una ruta rara,
// debe producir un error.
type Destination struct {
	environment string
}

// NewDestination construye el destino.
func NewDestination(environment string) (Destination, error) {
	if environment == "" {
		return Destination{}, fmt.Errorf("deployment: el contenido no tiene destino (ambiente)")
	}
	if environment == DestinationReserved {
		return Destination{}, fmt.Errorf(
			"deployment: %q está reservada y no puede ser un ambiente", DestinationReserved)
	}
	if strings.ContainsAny(environment, `/\:`) || environment == "." || environment == ".." {
		return Destination{}, fmt.Errorf(
			"deployment: el nombre de ambiente %q no puede usarse como destino", environment)
	}
	return Destination{environment: environment}, nil
}

// String es el nombre del ambiente.
func (d Destination) String() string { return d.environment }

// IsZero indica que no hay destino.
func (d Destination) IsZero() bool { return d.environment == "" }

// Equals es la regla de igualdad del value object.
func (d Destination) Equals(other Destination) bool { return d.environment == other.environment }
