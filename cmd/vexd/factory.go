package main

import (
	"os"

	"github.com/jairoprogramador/vex-engine/internal/application"
	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	defSvc "github.com/jairoprogramador/vex-engine/internal/domain/definition/services"
	execSvc "github.com/jairoprogramador/vex-engine/internal/domain/execution/services"
	stateSvc "github.com/jairoprogramador/vex-engine/internal/domain/state/services"
	verSvc "github.com/jairoprogramador/vex-engine/internal/domain/versioning/services"
	defInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/definition"
	execInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/execution"
	projInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/project"
	stateInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/state"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/storage/filesystem"
	verInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/versioning"
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
	// Infrastructure — storage
	execRepo := filesystem.NewExecutionRepository(cfg.storagePath)

	// Infrastructure — shared services
	gitCloner   := projInfra.NewGitClonerTemplate()
	gitRepo     := verInfra.NewGoGitRepository()
	stateRepo   := stateInfra.NewGobStateRepository()
	fpSvc       := stateInfra.NewSha256FingerprintService()
	runner      := execInfra.NewShellCommandRunner()
	copyWorkdir := execInfra.NewCopyWorkdir()
	varsRepo    := execInfra.NewGobVarsRepository()
	fs          := execInfra.NewOSFileSystem()
	projRepo    := projInfra.NewYAMLProjectRepository()
	defReader   := defInfra.NewYamlDefinitionReader()

	// Domain services
	interpolator    := execSvc.NewInterpolator()
	outputExtractor := execSvc.NewOutputExtractor()
	fileProcessor   := execSvc.NewFileProcessor(fs, interpolator)
	cmdExecutor     := execSvc.NewCommandExecutor(runner, fileProcessor, interpolator, outputExtractor)
	varResolver     := execSvc.NewVariableResolver(interpolator)
	stepExecutor    := execSvc.NewStepExecutor(cmdExecutor, varResolver)
	stateManager    := stateSvc.NewStateManager(stateRepo)
	versionCalc     := verSvc.NewVersionCalculator(gitRepo)
	planBuilder     := defSvc.NewPlanBuilder(defReader)

	// Adapters — bridging domain interface contracts
	pipelineParser := defInfra.NewPipelineParserAdapter(planBuilder)
	pipelineCloner := defInfra.NewPipelineClonerAdapter(gitCloner, cfg.rootVexPath+"/pipelines")

	// Application
	logBroker    := application.NewMemLogBroker()
	projectSvc   := application.NewProjectService(projRepo)
	workspaceSvc := application.NewWorkspaceService()
	orchestrator := application.NewExecutionOrchestrator(
		cfg.rootVexPath,
		projectSvc,
		workspaceSvc,
		gitCloner,
		versionCalc,
		pipelineParser,
		fpSvc,
		stateManager,
		stepExecutor,
		copyWorkdir,
		varsRepo,
		gitRepo,
		logBroker,
		execRepo,
	)

	// Use cases
	createExec       := usecase.NewCreateExecutionUseCase(orchestrator)
	getExec          := usecase.NewGetExecutionUseCase(execRepo)
	streamLogs       := usecase.NewLogsExecutionUseCase(logBroker, execRepo)
	deleteExec       := usecase.NewDeleteExecutionUseCase(orchestrator, execRepo)
	validatePipeline := usecase.NewValidatePipelineUseCase(pipelineCloner, pipelineParser)

	return vexhttp.NewServer(cfg.port, cfg.authToken, createExec, getExec, streamLogs, deleteExec, validatePipeline)
}
