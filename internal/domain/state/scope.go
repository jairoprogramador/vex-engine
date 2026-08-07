package state

import (
	"fmt"
	"strings"
)

// Los dos ámbitos posibles, y no hay un tercero (P13, spec 13).
//
//   - `project`: el trabajo es común a todos los ambientes del proyecto.
//   - `environment:<nombre>`: es propio del ambiente en ejecución.
const (
	scopeProjectToken     = "project"
	scopeEnvironmentToken = "environment"

	// scopeSeparator separa el token del nombre del ambiente en la forma
	// LÓGICA. La forma en RUTA no lo lleva: `:` es ilegal en rutas de Windows y
	// `vexd` se compila también para Windows (spec 02 §9.4, spec 11 §5.4). Por
	// eso `Segments()` existe y no es `strings.Split(String(), ":")`.
	scopeSeparator = ":"
)

// Scope es DIRECCIÓN, no contenido (diseño §7.7).
//
// Está en la clave de estado y NO en la huella: es lo que hace que el mismo
// trabajo, calculado con la misma huella, se recuerde en dos sitios distintos
// según dónde deba recordarse. La pregunta que responde la clave no es «¿son
// iguales las entradas?» sino «¿esto ya se ejecutó AQUÍ?».
//
// El aislamiento entre ambientes que la spec 10 compró metiendo el ambiente
// como séptima dimensión del hash se conserva por esta vía, con la misma
// garantía y una propiedad más: ya no puede caerse de un hash por descuido,
// porque no está en ninguno.
//
// El ambiente viaja SIEMPRE prefijado, así que un ambiente llamado `project`
// da `environment:project` y no colisiona con nada. La reserva de la palabra
// `shared` en `environments.yaml` (spec 04 §5.4) sigue viva mientras el almacén
// viejo exista; se retira con la spec 13.
type Scope struct {
	kind        string
	environment string
}

// NewProjectScope es el ámbito común a todos los ambientes del proyecto.
func NewProjectScope() Scope {
	return Scope{kind: scopeProjectToken}
}

// NewEnvironmentScope es el ámbito de un ambiente concreto.
//
// El nombre se valida contra lo que puede romper una ruta —separadores y
// travesía de directorios— porque el ámbito ES un tramo de la ruta del almacén.
// Un nombre que la rompa no debe producir una ruta rara: debe producir un error.
func NewEnvironmentScope(environment string) (Scope, error) {
	if environment == "" {
		return Scope{}, fmt.Errorf("state: el ámbito de ambiente no tiene nombre")
	}
	if strings.ContainsAny(environment, `/\`+scopeSeparator) || environment == "." || environment == ".." {
		return Scope{}, fmt.Errorf(
			"state: el nombre de ambiente %q no puede usarse como ámbito", environment)
	}
	return Scope{kind: scopeEnvironmentToken, environment: environment}, nil
}

// ParseScope lee la forma lógica: "project" o "environment:<nombre>". La
// necesita el índice, que guarda la clave de estado a la que apunta.
func ParseScope(text string) (Scope, error) {
	if text == scopeProjectToken {
		return NewProjectScope(), nil
	}
	token, environment, found := strings.Cut(text, scopeSeparator)
	if !found || token != scopeEnvironmentToken {
		return Scope{}, fmt.Errorf(
			"state: %q no es un ámbito (\"project\" o \"environment:<nombre>\")", text)
	}
	return NewEnvironmentScope(environment)
}

// String es la forma lógica y externa del ámbito.
func (s Scope) String() string {
	if s.IsZero() {
		return ""
	}
	if s.kind == scopeProjectToken {
		return scopeProjectToken
	}
	return scopeEnvironmentToken + scopeSeparator + s.environment
}

// Segments es la forma en RUTA: un segmento para `project`, DOS para un
// ambiente. Ver el comentario de scopeSeparator.
func (s Scope) Segments() []string {
	if s.IsZero() {
		return nil
	}
	if s.kind == scopeProjectToken {
		return []string{scopeProjectToken}
	}
	return []string{scopeEnvironmentToken, s.environment}
}

// IsProject dice si el ámbito es el común del proyecto.
func (s Scope) IsProject() bool { return s.kind == scopeProjectToken }

// Environment es el nombre del ambiente, vacío en el ámbito de proyecto.
func (s Scope) Environment() string { return s.environment }

// IsZero indica que no hay ámbito.
func (s Scope) IsZero() bool {
	return s.kind == "" || (s.kind == scopeEnvironmentToken && s.environment == "")
}

// Equals es la regla de igualdad del value object.
func (s Scope) Equals(other Scope) bool {
	return s.kind == other.kind && s.environment == other.environment
}
