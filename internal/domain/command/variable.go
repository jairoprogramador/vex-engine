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

type Variable struct {
	name     string
	value    string
	isShared bool
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
func NewVariable(name, value string, isShared bool) (Variable, error) {
	if name == "" {
		return Variable{}, ErrVariableNameEmpty
	}

	return Variable{
		name:     name,
		value:    value,
		isShared: isShared,
	}, nil
}

func (ve *Variable) Name() string {
	return ve.name
}

func (ve *Variable) Value() string {
	return ve.value
}

func (ve *Variable) IsShared() bool {
	return ve.isShared
}
