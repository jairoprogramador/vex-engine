package pipeline

import (
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

// PipelineURL identifica el repositorio Git del pipeline (valor de dominio pipeline).
type PipelineURL struct {
	shared.RepoURL
}

func NewPipelineURL(raw string) (PipelineURL, error) {
	u, err := shared.ParseRepoURL(raw)
	if err != nil {
		return PipelineURL{}, err
	}
	return PipelineURL{RepoURL: u}, nil
}
