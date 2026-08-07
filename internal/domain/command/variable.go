package command

import (
	"errors"
	"slices"
)

const (
	// `shared_workdir` vivía aquí, declarada y sin que nadie la asignara jamás
	// (D6). Se elimina con la spec 11: una variable que no existe no puede ser
	// volátil ni no volátil, y conservarla es invitar a que alguien la asigne sin
	// saber qué significaba.
	VarStepWorkdir         = "step_workdir"
	VarEnvironment         = "environment"
	VarProjectID           = "project_id"
	VarProjectName         = "project_name"
	VarProjectOrg          = "project_organization"
	VarProjectTeam         = "project_team"
	VarProjectWorkdir      = "project_workdir"
	VarProjectVersion      = "project_version"
	VarProjectRevision     = "project_revision"
	VarProjectRevisionFull = "project_revision_full"
	VarToolName            = "tool_name"
)

// volatileVarNames son las variables que el motor DERIVA de cada ejecución y
// que por tanto no describen la intención de nadie: cambian solas.
//
// Ni entran en la identidad de un step —harían que ningún step se saltara
// jamás— ni se guardan en el almacén, que las recalcularía mal en la corrida
// siguiente. Las dos exclusiones eran la misma lista escrita dos veces, en
// `vars_rule.go` y en `step_executable.go`, sin nada que las mantuviera
// sincronizadas; ahora hay un solo dueño (spec 10).
//
// La lista es NORMATIVA para la huella de variables: está transcrita en
// `fingerprint/SPEC-VARIABLES-v1.md` §3.1, y `TestVolatileVarNames_...` la fija.
var volatileVarNames = []string{
	VarProjectVersion,
	VarProjectRevision,
	VarProjectRevisionFull,
	VarToolName,
	VarProjectWorkdir,
	VarStepWorkdir,
}

// VolatileVarNames devuelve una copia de la lista de variables derivadas.
func VolatileVarNames() []string {
	names := make([]string, len(volatileVarNames))
	copy(names, volatileVarNames)
	return names
}

// IsVolatileVar dice si una variable la deriva el motor de la ejecución en
// curso.
func IsVolatileVar(name string) bool {
	return slices.Contains(volatileVarNames, name)
}

// Variable es un par nombre/valor del mapa acumulado, con la marca de dónde
// vino.
//
// `isShared` vivía aquí y se retira con la spec 13 §5.6: el ámbito es del STEP,
// así que la marca por variable era información duplicada —todas las que un step
// produce tienen el suyo— y fue la fuente del defecto que la spec 02 tuvo que
// corregir, cuando el adaptador de archivo la perdía y el mismo proyecto
// producía una huella distinta según corriera en local o en remoto. Aquel
// arreglo no se revierte: se queda sin objeto, porque el dato que se perdía ya
// no viaja por aquí.
type Variable struct {
	name  string
	value string

	// origin es de dónde llegó el valor, y con ello su precedencia frente a otra
	// fuente que aporte el mismo nombre (spec 12 §5.1).
	//
	// NO entra en la huella de variables: lo que identifica es el par
	// (nombre, valor) resultante, no por dónde llegó. Dos ejecuciones que llegan
	// al mismo valor por caminos distintos son la misma configuración —la misma
	// regla que P9 aplica al código: la huella identifica, el commit documenta—.
	// Está escrito en `fingerprint/SPEC-VARIABLES-v1.md` §4, que es donde lo
	// buscará quien lea la especificación de la huella.
	origin Origin
}

// ErrVariableNameEmpty es el único invariante que le queda a Variable. Un
// nombre vacío no puede venir de datos: las diez variables iniciales llevan
// nombres literales del propio motor y las declaradas llevan el `name:` del
// pipelinecode, así que un nombre vacío es siempre un defecto del llamador y
// debe abortar la ejecución (spec 03 §5.1).
var ErrVariableNameEmpty = errors.New("el nombre de la variable generada no puede estar vacío")

// NewVariable ya NO rechaza el valor vacío. Un campo del proyecto sin rellenar
// —una organización sin valor, un equipo sin asignar— o un parámetro declarado
// sin valor son información legítima, no un error (spec 03 §5.1, cierra D-A8).
//
// «Declarada y vacía» y «no declarada» siguen siendo estados distintos: el
// material canónico de la huella serializa Quote(name) y Quote(value), y
// Quote("") es `""`, no ausencia.
//
// El `origin` es obligatorio y no tiene valor por defecto útil: el valor cero
// del enum es `OriginDeclared`, la precedencia MÁS BAJA, así que un llamador que
// se olvide de declararlo produce una variable que no pisa a nadie en vez de una
// que lo pisa todo. Es la dirección segura del olvido.
func NewVariable(name, value string, origin Origin) (Variable, error) {
	if name == "" {
		return Variable{}, ErrVariableNameEmpty
	}

	return Variable{
		name:   name,
		value:  value,
		origin: origin,
	}, nil
}

func (ve *Variable) Name() string {
	return ve.name
}

func (ve *Variable) Value() string {
	return ve.value
}

func (ve *Variable) Origin() Origin {
	return ve.origin
}
