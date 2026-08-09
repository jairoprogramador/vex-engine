package deployment

import (
	"fmt"
	"strings"
)

// AQUÍ vivía `DestinationReserved = "shared"`, y con él una reserva que la
// spec 13 §5.5 ya había DEROGADO cuando la 17 la reintrodujo.
//
// El argumento de la 17 era que «la reserva sobrevive al rediseño del ámbito»;
// el código dice lo contrario y la 13 lo dice por escrito: el ámbito de ambiente
// viaja SIEMPRE prefijado en la clave de estado —`environment:<nombre>`— así que
// un ambiente llamado `shared` produce `environment:shared` y no colisiona con
// nada. **No queda ninguna palabra reservada en el vocabulario del usuario**, y
// hay un test de integración que lo afirma
// (`TestRunCommand_NingunaPalabraDelUsuarioEstaReservada`).
//
// La spec 18 es donde se vio, porque es la primera que construye un
// `Destination` en el camino de ejecución: hasta aquí el tipo sólo existía en
// sus propios tests. Un ambiente `shared` desplegaba bien y dejaba de hacerlo al
// cablear el resolutor.

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
