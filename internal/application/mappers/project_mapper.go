package mappers

/* import (
	"fmt"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
)

func MapToProject(cmd dto.RequestInput) (*project.Project, error) {
	id, err := projDomain.NewProjectID(cmd.Project.Id)
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
	url, err := projDomain.NewProjectUrl(cmd.Project.Url)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	ref, err := projDomain.NewProjectRef(cmd.Project.Ref)
	if err != nil {
		return nil, fmt.Errorf("project: %w", err)
	}
	return projDomain.NewProject(id, name, team, org, url, ref)
} */
