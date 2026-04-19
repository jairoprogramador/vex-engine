package project

import "errors"

type ProjectID struct{ value string }

func NewProjectID(id string) (ProjectID, error) {
	if id == "" {
		return ProjectID{}, errors.New("project_id no puede estar vacío")
	}
	return ProjectID{value: id}, nil
}

func (p ProjectID) String() string          { return p.value }
func (p ProjectID) Equals(other ProjectID) bool { return p.value == other.value }
