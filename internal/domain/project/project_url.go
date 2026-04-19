package project

import (
	"github.com/jairoprogramador/vex-engine/internal/domain/shared"
)

// ProjectURL identifica el repositorio Git del proyecto (valor de dominio project).
type ProjectURL struct {
	shared.RepoURL
}

func NewProjectURL(raw string) (ProjectURL, error) {
	u, err := shared.ParseRepoURL(raw)
	if err != nil {
		return ProjectURL{}, err
	}
	return ProjectURL{RepoURL: u}, nil
}
