package main

import (
	"os"

	"github.com/jairoprogramador/vex-engine/internal/application"
	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	execSvc "github.com/jairoprogramador/vex-engine/internal/domain/execution/services"
	pipSer "github.com/jairoprogramador/vex-engine/internal/domain/pipeline/services"
	projServices "github.com/jairoprogramador/vex-engine/internal/domain/project/services"
	stateSvc "github.com/jairoprogramador/vex-engine/internal/domain/state/services"
	execInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/execution"
	gitInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/git"
	pipInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	projInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/project"
	stateInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/state"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/storage/filesystem"
	vexhttp "github.com/jairoprogramador/vex-engine/internal/interfaces/http"
)

// config agrupa la configuración del daemon leída desde env vars.
type config struct {
	port        string
	storagePath string
	rootVexPath string
	authToken   string
}

// loadConfig lee las variables de entorno y aplica valores por defecto.
func loadConfig() config {
	return config{
		port:        envOrDefault("VEXD_PORT", "8080"),
		storagePath: envOrDefault("VEXD_STORAGE_PATH", "/var/lib/vexd"),
		rootVexPath: envOrDefault("VEXD_ROOT_PATH", "/var/lib/vexd"),
		authToken:   os.Getenv("VEXD_AUTH_TOKEN"),
	}
}

// buildServer construye y cablea todas las dependencias del daemon y retorna el servidor HTTP listo.
func buildServer(cfg config) *vexhttp.Server {
	pipelinesBaseDir := cfg.rootVexPath + "/pipelines"
	projectsBaseDir := cfg.rootVexPath + "/projects"

	// Infrastructure — storage
	execRepo := filesystem.NewExecutionRepository(cfg.storagePath)

	// Infrastructure — shared services
	runner := execInfra.NewShellCommandRunner()
	gitCloner := gitInfra.NewRepositoryGitImpl()
	stateRepo := stateInfra.NewGobStateRepository()
	fpSvc := stateInfra.NewSha256FingerprintService()
	copyWorkdir := execInfra.NewCopyWorkdir()
	varsRepo := execInfra.NewGobVarsRepository()
	fs := execInfra.NewOSFileSystem()

	// Infrastructure — project
	projectRepo := projInfra.NewGitRepositoryFetcher(gitCloner, projectsBaseDir)

	// Infrastructure — pipeline
	gitFetcher := pipInfra.NewGitRepositoryFetcher(gitCloner, pipelinesBaseDir)
	pipelineLoader := pipSer.NewPlanResolver(pipInfra.NewYamlPipelineReader())

	// Domain services
	interpolator := execSvc.NewInterpolator()
	outputExtractor := execSvc.NewOutputExtractor()
	fileProcessor := execSvc.NewFileProcessor(fs, interpolator)
	cmdExecutor := execSvc.NewCommandExecutor(runner, fileProcessor, interpolator, outputExtractor)
	varResolver := execSvc.NewVariableResolver(interpolator)
	stepExecutor := execSvc.NewStepExecutor(cmdExecutor, varResolver)
	stateManager := stateSvc.NewStateManager(stateRepo)
	versionResolver := projServices.NewVersionResolver(projectRepo)

	// Application
	logBroker := application.NewMemLogBroker()
	projectSvc := application.NewProjectService()
	workspaceSvc := application.NewWorkspaceService()
	orchestrator := application.NewExecutionOrchestrator(
		cfg.rootVexPath,
		projectSvc,
		workspaceSvc,
		gitCloner,
		versionResolver,
		projectRepo,
		gitFetcher,
		pipelineLoader,
		fpSvc,
		stateManager,
		stepExecutor,
		copyWorkdir,
		varsRepo,
		logBroker,
		execRepo,
	)

	// Use cases
	createExec := usecase.NewCreateExecutionUseCase(orchestrator)
	getExec := usecase.NewGetExecutionUseCase(execRepo)
	streamLogs := usecase.NewLogsExecutionUseCase(logBroker, execRepo)
	deleteExec := usecase.NewDeleteExecutionUseCase(orchestrator, execRepo)
	validatePipeline := usecase.NewValidatePipelineUseCase(gitFetcher, pipelineLoader)

	return vexhttp.NewServer(cfg.port, cfg.authToken, createExec, getExec, streamLogs, deleteExec, validatePipeline)
}
