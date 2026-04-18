package pipeline

import "errors"

type RepositoryRef struct {
	ref string
}

func NewRepositoryRef(ref string) (RepositoryRef, error) {
	if ref == "" {
		return RepositoryRef{}, errors.New("la referencia del repositorio no puede estar vacía")
	}
	return RepositoryRef{ref: ref}, nil
}

func (r RepositoryRef) String() string {
	return r.ref
}
