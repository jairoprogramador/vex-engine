package project

import "errors"

type ProjectOrg struct{ value string }

func NewProjectOrg(org string) (ProjectOrg, error) {
	if org == "" {
		return ProjectOrg{}, errors.New("project_org no puede estar vacío")
	}
	return ProjectOrg{value: org}, nil
}

func (p ProjectOrg) String() string { return p.value }
