package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	pipDom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	stepDom "github.com/jairoprogramador/vex-engine/internal/domain/step"
	stepStat "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
	cmdInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/command"
	pippInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	stepInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/step"
	stepStatInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/step/status"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/cli"
)

// vexdConfig agrupa la configuración mínima del binario one-shot.
type vexdConfig struct {
	rootVexPath string
}

func loadConfig() (vexdConfig, error) {
	root := os.Getenv("VEXD_ROOT_PATH")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return vexdConfig{}, fmt.Errorf("resolve user home: %w", err)
		}
		root = home
	}
	return vexdConfig{rootVexPath: root}, nil
}

// buildRunCommand cablea todas las dependencias y retorna un *cli.RunCommand
// listo para usar. Lo invoca el subcomando `vexd run` por cada ejecución.
//
// El wiring NO incluye observers: éstos los construye RunCommand.Execute
// dinámicamente según los flags (--log-endpoint, --status-endpoint, --quiet).
func buildRunCommand() (*cli.RunCommand, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
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
	createExec := usecase.NewCreateExecutionUseCase(
		executablePipeline,
		executableCommand,
		executableStep,
	)

	return cli.NewRunCommand(createExec), nil
}

func chainPipelineHandlers(handlers ...pipDom.PipelineHandler) pipDom.PipelineHandler {
	if len(handlers) == 0 {
		return nil
	}
	for i := 0; i < len(handlers)-1; i++ {
		handlers[i].SetNext(handlers[i+1])
	}
	return handlers[0]
}

func chainStepHandlers(handlers ...stepDom.StepHandler) stepDom.StepHandler {
	if len(handlers) == 0 {
		return nil
	}
	for i := 0; i < len(handlers)-1; i++ {
		handlers[i].SetNext(handlers[i+1])
	}
	return handlers[0]
}

func chainCommandHandlers(handlers ...command.CommandHandler) command.CommandHandler {
	if len(handlers) == 0 {
		return nil
	}
	for i := 0; i < len(handlers)-1; i++ {
		handlers[i].SetNext(handlers[i+1])
	}
	return handlers[0]
}
