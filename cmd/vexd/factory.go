package main

import (
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	pipDom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	stepDom "github.com/jairoprogramador/vex-engine/internal/domain/step"
	stepStat "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
	cmdInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/command"
	notify "github.com/jairoprogramador/vex-engine/internal/infrastructure/notify"
	pippInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	stepInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/step"
	stepStatInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/step/status"
	vexhttp "github.com/jairoprogramador/vex-engine/internal/interfaces/http"
)

// config agrupa la configuración del daemon leída desde env vars.
type config struct {
	port             string
	rootVexPath      string
	authToken        string
	logStdoutEnabled bool
}

// loadConfig lee las variables de entorno y aplica valores por defecto.
func loadConfig() config {
	home := userHomeDir()
	return config{
		port:             envOrDefault("VEXD_PORT", "65001"),
		rootVexPath:      envOrDefault("VEXD_ROOT_PATH", home),
		authToken:        os.Getenv("VEXD_AUTH_TOKEN"),
		logStdoutEnabled: os.Getenv("VEXD_LOG_STDOUT") == "1",
	}
}

func chainPipelineHandlers(handlers ...pipDom.PipelineHandler) pipDom.PipelineHandler {
	for i := 0; i < len(handlers)-1; i++ {
		handlers[i].SetNext(handlers[i+1])
	}
	if len(handlers) == 0 {
		return nil
	}
	return handlers[0]
}

func chainStepHandlers(handlers ...stepDom.StepHandler) stepDom.StepHandler {
	for i := 0; i < len(handlers)-1; i++ {
		handlers[i].SetNext(handlers[i+1])
	}
	if len(handlers) == 0 {
		return nil
	}
	return handlers[0]
}

func chainCommandHandlers(handlers ...command.CommandHandler) command.CommandHandler {
	for i := 0; i < len(handlers)-1; i++ {
		handlers[i].SetNext(handlers[i+1])
	}
	if len(handlers) == 0 {
		return nil
	}
	return handlers[0]
}

// buildServer construye y cablea todas las dependencias del daemon y retorna el servidor HTTP listo.
// Opcional (VEXD_LOG_STDOUT=1): registra StdoutLogObserver en el mismo publicador sin perder SSE.
func buildServer(cfg config) *vexhttp.Server {
	projectsBasePath := filepath.Join(cfg.rootVexPath, ".vex", "projects")

	// --- Infrastructure: pipeline ---
	projectClonerRepo := pippInfra.NewProjectClonerRepository(projectsBasePath)
	pipelineClonerRepo := pippInfra.NewPipelineClonerRepository(projectsBasePath)
	pipelineEnvRepo := pippInfra.NewPipelineEnvironmentRepository()
	pipelineStepRepo := pippInfra.NewPipelineStepRepository()
	pipelineWorkdirRepo := pippInfra.NewPipelineWorkdirRepository(projectsBasePath)
	projectTagRepo := pippInfra.NewProjectTagRepository()
	projectFingerprint := pippInfra.NewProjectFingerprint()

	// --- Infrastructure: step (vars, commands, status) ---
	varsStoreRepo := stepInfra.NewFileVarsStoreRepository(projectsBasePath)
	pipelineVarsRepo := stepInfra.NewPipelineVarsRepository()
	pipelineCommandRepo := stepInfra.NewPipelineCommandRepository()
	instStatusRepo := stepStatInfra.NewFileInstStatusRepository(projectsBasePath)
	varsStatusRepo := stepStatInfra.NewFileVarsStatusRepository(projectsBasePath)
	codeStatusRepo := stepStatInfra.NewFileCodeStatusRepository(projectsBasePath)
	timeStatusRepo := stepStatInfra.NewFileTimeStatusRepository(projectsBasePath)

	// --- Infrastructure: command (shell, filesystem) ---
	fileSystem := cmdInfra.NewFileSystemManager()
	shellRunner := cmdInfra.NewShellCommandRunner()

	// --- Domain: pipeline handler chain (orden 01 → 09) ---
	pipelineHead := chainPipelineHandlers(
		pipDom.NewProjectClonerHandler(projectClonerRepo),
		pipDom.NewPipelineClonerHandler(pipelineClonerRepo),
		pipDom.NewEnvironmentLoaderHandler(pipelineEnvRepo),
		pipDom.NewStepsLoaderHandler(pipelineStepRepo),
		pipDom.NewCopyWorkdirHandler(pipelineWorkdirRepo),
		pipDom.NewVersionCalculatorHandler(projectTagRepo),
		pipDom.NewInitVarsHandler(),
		pipDom.NewProjectStatusHandler(projectFingerprint),
		pipDom.NewPipelineRunnerHandler(),
	)
	executablePipeline := pipDom.NewPipelineExecutable(pipelineHead)

	// --- Domain: policy registry (step runner) ---
	ruleRegistry := stepStat.NewRuleRegistry()
	ruleRegistry.Register(stepStat.NewInstructionsPipelineRule(instStatusRepo))
	ruleRegistry.Register(stepStat.NewVariablesRuleRule(varsStatusRepo))
	ruleRegistry.Register(stepStat.NewCodeProjectRuleRule(codeStatusRepo))
	ruleRegistry.Register(stepStat.NewTimeRule(timeStatusRepo))
	policyBuilder := stepStat.NewPolicyBuilder(ruleRegistry)

	// --- Domain: step handler chain ---
	stepHead := chainStepHandlers(
		stepDom.NewVarsStoreSharedHandler(varsStoreRepo),
		stepDom.NewVarsStoreStepHandler(varsStoreRepo),
		stepDom.NewVarsHandler(pipelineVarsRepo),
		stepDom.NewStepRunnerHandler(pipelineCommandRepo, policyBuilder),
	)
	statusRepo := stepStatInfra.NewFileStatusRepository(varsStatusRepo, timeStatusRepo, instStatusRepo, codeStatusRepo)
	executableStep := stepDom.NewStepExecutable(stepHead, varsStoreRepo, statusRepo)

	// --- Domain: command handler chain ---
	fileInterpolator := command.NewFileInterpolator(fileSystem)
	commandHead := chainCommandHandlers(
		command.NewFilesInterpolatorHandler(*fileInterpolator),
		command.NewCommandInterpolatorHandler(),
		command.NewCommandRunnerHandler(shellRunner),
		command.NewRegexCheckerHandler(),
		command.NewVarsExtractorHandler(),
	)
	executableCommand := command.NewCommandExecutable(commandHead)

	// --- Application ---
	logBroker := notify.NewMemLogPublisher()
	if cfg.logStdoutEnabled {
		logBroker.RegisterObserver(notify.NewStdoutLogObserver())
	}

	createExec := usecase.NewCreateExecutionUseCase(
		executablePipeline,
		executableCommand,
		executableStep,
		logBroker,
	)
	getExec := usecase.NewGetExecutionUseCase()
	streamLogs := usecase.NewLogsExecutionUseCase(logBroker)
	deleteExec := usecase.NewDeleteExecutionUseCase()
	validatePipeline := usecase.NewValidatePipelineUseCase(projectsBasePath)

	return vexhttp.NewServer(cfg.port, cfg.authToken, createExec, getExec, streamLogs, deleteExec, validatePipeline)
}
