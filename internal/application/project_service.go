package application

import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	projDomain "github.com/jairoprogramador/vex-engine/internal/domain/project"
)

type ProjectService struct{}

func NewProjectService() *ProjectService { return &ProjectService{} }

func (s *ProjectService) FromDTO(cmd dto.RequestInput) (*projDomain.Project, error) {
	id, err := projDomain.NewProjectID(cmd.Project.ID)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	name, err := projDomain.NewProjectName(cmd.Project.Name)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	team, err := projDomain.NewProjectTeam(cmd.Project.Team)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	org, err := projDomain.NewProjectOrg(cmd.Project.Org)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	url, err := projDomain.NewProjectURL(cmd.Project.URL)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	ref, err := projDomain.NewProjectRef(cmd.Project.Ref)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	return projDomain.NewProject(id, name, team, org, url, ref)
}
