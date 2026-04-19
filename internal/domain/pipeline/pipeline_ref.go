package pipeline

import "errors"

type PipelineRef struct {
	ref string
}

func NewPipelineRef(ref string) (PipelineRef, error) {
	if ref == "" {
		return PipelineRef{}, errors.New("la referencia del repositorio no puede estar vacía")
	}
	return PipelineRef{ref: ref}, nil
}

func (r PipelineRef) String() string {
	return r.ref
}
