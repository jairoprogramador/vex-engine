package project

import "errors"

type ProjectRef struct {
	ref string
}

func NewProjectRef(ref string) (ProjectRef, error) {
	if ref == "" {
		return ProjectRef{}, errors.New("project_ref no puede estar vacío")
	}
	return ProjectRef{ref: ref}, nil
}

func (r ProjectRef) String() string { return r.ref }
