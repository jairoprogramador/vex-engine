package project

import "errors"

type ProjectTeam struct{ value string }

func NewProjectTeam(team string) (ProjectTeam, error) {
	if team == "" {
		return ProjectTeam{}, errors.New("project_team no puede estar vacío")
	}
	return ProjectTeam{value: team}, nil
}

func (p ProjectTeam) String() string { return p.value }
