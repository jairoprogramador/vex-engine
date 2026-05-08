package command

type ExecutionProject struct {
	projectId        string
	projectName      string
	projectUrl       string
	projectRef       string
	projectOrg       string
	projectTeam      string
	projectStatus    string
	projectLocalPath string
}

func NewExecutionProject(
	projectId, projectName, projectUrl, projectRef,
	projectOrg, projectTeam string) ExecutionProject {
	return ExecutionProject{
		projectId:        projectId,
		projectName:      projectName,
		projectUrl:       projectUrl,
		projectRef:       projectRef,
		projectOrg:       projectOrg,
		projectTeam:      projectTeam,
		projectLocalPath: "",
		projectStatus:    "",
	}
}

func (e *ExecutionProject) ProjectId() string {
	return e.projectId
}

func (e *ExecutionProject) ProjectName() string {
	return e.projectName
}

func (e *ExecutionProject) ProjectOrg() string {
	return e.projectOrg
}

func (e *ExecutionProject) ProjectTeam() string {
	return e.projectTeam
}

func (e *ExecutionProject) ProjectUrl() string {
	return e.projectUrl
}

func (e *ExecutionProject) ProjectRef() string {
	return e.projectRef
}

func (e *ExecutionProject) SetProjectLocalPath(projectLocalPath string) {
	e.projectLocalPath = projectLocalPath
}

func (e *ExecutionProject) ProjectLocalPath() string {
	return e.projectLocalPath
}

func (e *ExecutionProject) ProjectStatus() string {
	return e.projectStatus
}

func (e *ExecutionProject) SetProjectStatus(projectStatus string) {
	e.projectStatus = projectStatus
}
