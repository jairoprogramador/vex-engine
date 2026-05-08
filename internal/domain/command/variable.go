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

func NewVariable(name, value string, isShared bool) (Variable, error) {
	if name == "" {
		return Variable{}, errors.New("el nombre de la variable generada no puede estar vacío")
	}
	if value == "" {
		return Variable{}, errors.New("el valor de la variable generada no puede estar vacío")
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
