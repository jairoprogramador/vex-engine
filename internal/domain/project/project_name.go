package project

import "errors"

type ProjectName struct{ value string }

func NewProjectName(name string) (ProjectName, error) {
	if name == "" {
		return ProjectName{}, errors.New("project_name no puede estar vacío")
	}
	return ProjectName{value: name}, nil
}

func (p ProjectName) String() string { return p.value }
