package application

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/domain/project/aggregates"
	"github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
	"github.com/jairoprogramador/vex-engine/internal/domain/project/vos"
)

type ProjectService struct {
	projectRepo ports.ProjectRepository
}

func NewProjectService(
	projectRepo ports.ProjectRepository) *ProjectService {

	return &ProjectService{
		projectRepo: projectRepo,
	}
}

// Load lee el archivo vexconfig.yaml en projectLocalPath y construye el agregado Project.
// Si el ID almacenado no coincide con el generado a partir de los datos del proyecto,
// lo sincroniza silenciosamente y persiste el cambio.
func (s *ProjectService) Load(
	ctx context.Context, projectLocalPath string) (*aggregates.Project, error) {
	projectConfigPath := filepath.Join(projectLocalPath, "vexconfig.yaml")

	projectDTO, err := s.projectRepo.Load(ctx, projectConfigPath)
	if err != nil {
		return nil, fmt.Errorf("no se pudo cargar la configuración del proyecto: %w", err)
	}

	projectData, err := vos.NewProjectData(
		projectDTO.Name, projectDTO.Organization,
		projectDTO.Team, projectDTO.Description, projectDTO.Version)
	if err != nil {
		return nil, fmt.Errorf("datos del proyecto inválidos: %w", err)
	}

	templateRepo, err := vos.NewTemplateRepository(projectDTO.TemplateURL, projectDTO.TemplateRef)
	if err != nil {
		return nil, fmt.Errorf("datos del repositorio de plantillas inválidos: %w", err)
	}

	projectID := vos.NewProjectID(projectDTO.ID)
	project := aggregates.NewProject(projectID, projectData, templateRepo, projectLocalPath)

	if project.SyncID() {
		projectDTO.ID = project.ID().String()
		if err := s.projectRepo.Save(ctx, projectConfigPath, projectDTO); err != nil {
			return nil, fmt.Errorf("no se pudo guardar el ID del proyecto actualizado: %w", err)
		}
	}

	return project, nil
}

// FromDTO construye un agregado Project directamente desde un CreateExecutionCommand,
// sin tocar el filesystem. Útil cuando los datos del proyecto vienen del request HTTP
// en lugar de un vexconfig.yaml local.
func (s *ProjectService) FromDTO(cmd dto.CreateExecutionCommand, projectLocalPath string) (*aggregates.Project, error) {
	projectData, err := vos.NewProjectData(cmd.Project.Name, cmd.Project.Org, cmd.Project.Team, "", "")
	if err != nil {
		return nil, fmt.Errorf("datos del proyecto inválidos: %w", err)
	}

	templateRepo, err := vos.NewTemplateRepository(cmd.Pipeline.URL, cmd.Pipeline.Ref)
	if err != nil {
		return nil, fmt.Errorf("datos del repositorio de plantillas inválidos: %w", err)
	}

	projectID := vos.NewProjectID(cmd.Project.ID)
	project := aggregates.NewProject(projectID, projectData, templateRepo, projectLocalPath)
	project.SyncID()

	return project, nil
}
