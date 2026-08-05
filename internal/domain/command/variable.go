package command

import "errors"

const (
	VarStepWorkdir         = "step_workdir"
	VarSharedWorkdir       = "shared_workdir"
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
