package application

import (
	"github.com/jairoprogramador/vex/internal/domain/workspace/aggregates"
	"github.com/jairoprogramador/vex/internal/domain/workspace/vos"
)

type WorkspaceService struct{}

func NewWorkspaceService() *WorkspaceService {
	return &WorkspaceService{}
}

func (s *WorkspaceService) NewWorkspace(rootVexPath, projectName, templateName string) (*aggregates.Workspace, error) {
	wsRootPath, err := vos.NewRootPath(rootVexPath)
	if err != nil {
		return nil, err
	}

	wsProjectName, err := vos.NewProjectName(projectName)
	if err != nil {
		return nil, err
	}

	wsTemplateName, err := vos.NewTemplateName(templateName)
	if err != nil {
		return nil, err
	}

	return aggregates.NewWorkspace(wsRootPath, wsProjectName, wsTemplateName)
}
