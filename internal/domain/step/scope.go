package step

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// Los dos ámbitos que un step puede DECLARAR, y no hay un tercero (spec 13 §5.1).
//
//   - `project`: el trabajo es común a todos los ambientes del proyecto.
//   - `environment`: el trabajo es propio del ambiente en ejecución.
const (
	ScopeProjectToken     = "project"
	ScopeEnvironmentToken = "environment"
)

// Scope es el ámbito que un step declara en su `config.yaml`, y es un atributo
// del STEP: ni de su ruta de trabajo ni de un comando suyo (spec 13 §5.1').
//
// Aquí murió `CommandWorkdir.scope()`, que deducía el ámbito del PRIMER segmento
// de `workdir`. La regla posicional no falló por estar mal escrita: falló porque
// un mismo string decidía tres cosas independientes —directorio de ejecución,
// base de resolución de plantillas y ámbito—, así que reorganizar los `.tf` bajo
// `terraform/shared` desactivó el ámbito compartido sin que nadie lo relacionara.
// Estuvo muerto en producción desde entonces y nadie lo notó (spec 13 §1).
//
// El concepto tiene DOS papeles y conviene no confundirlos:
//
//   - como VALOR DECLARADO, `scope: project` es contenido del `config.yaml` y
//     entra en la huella como cualquier otro campo (spec 27 §5.2bis);
//   - como DIRECCIÓN, determina la ruta donde vive el registro (`StateScope`), y
//     esa ruta no entra en ningún hash.
//
// Que el value object exista en un solo sitio es lo que permite que las dos
// lecturas salgan del mismo dato en vez de divergir — que es exactamente lo que
// pasó con `shared`, que significaba tres cosas y ninguna tenía tipo.
type Scope struct {
	token string
}

// NewScope construye el ámbito a partir de lo declarado. El vocabulario es
// CERRADO: un ámbito inventado no tendría dónde persistirse, así que un valor
// fuera de los dos es un error de pipelinecode y no un ámbito nuevo.
//
// La cadena vacía tiene su propio mensaje porque su causa es otra: un
// `config.yaml` presente que no declara `scope`, que es un olvido y no un typo.
func NewScope(text string) (Scope, error) {
	switch text {
	case ScopeProjectToken, ScopeEnvironmentToken:
		return Scope{token: text}, nil
	case "":
		return Scope{}, fmt.Errorf(
			"no declara 'scope': se espera %q o %q",
			ScopeProjectToken, ScopeEnvironmentToken)
	default:
		return Scope{}, fmt.Errorf(
			"'scope: %s' no es un ámbito: se espera %q o %q",
			text, ScopeProjectToken, ScopeEnvironmentToken)
	}
}

// NewProjectScope y NewEnvironmentScope son los dos ámbitos por su nombre, para
// quien los construye sin pasar por una cadena.
func NewProjectScope() Scope     { return Scope{token: ScopeProjectToken} }
func NewEnvironmentScope() Scope { return Scope{token: ScopeEnvironmentToken} }

// String es la forma declarada, la misma que se escribe en el `config.yaml`.
func (s Scope) String() string { return s.token }

// IsZero indica que no hay ámbito declarado.
func (s Scope) IsZero() bool { return s.token == "" }

// IsProject dice si el ámbito es el común del proyecto.
func (s Scope) IsProject() bool { return s.token == ScopeProjectToken }

// Equals es la regla de igualdad del value object.
func (s Scope) Equals(other Scope) bool { return s.token == other.token }

// StateScope traduce el ámbito DECLARADO a la DIRECCIÓN donde vive el registro
// (spec 11 §5.1). Es el segundo de los dos papeles del concepto, y el único
// punto donde uno se convierte en el otro.
//
// El ambiente hace falta sólo para el ámbito de ambiente; para el de proyecto se
// ignora, que es justo lo que hace que dos despliegues a ambientes distintos
// escriban en el MISMO sitio.
//
// La validación del nombre del ambiente NO se retira con la reserva de `shared`
// (spec 13 §5.5): `state.NewEnvironmentScope` sigue rechazando `/`, `\`, `:`,
// `.` y `..`, que es lo que impide que un `value: pro/duccion` produzca un
// directorio anidado dentro del almacén.
func (s Scope) StateScope(environment string) (state.Scope, error) {
	if s.IsZero() {
		return state.Scope{}, fmt.Errorf("step: el step no declara ámbito")
	}
	if s.IsProject() {
		return state.NewProjectScope(), nil
	}
	return state.NewEnvironmentScope(environment)
}
