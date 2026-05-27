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

const vexHomeMountPoint = "/vexHome"
const vexHomeDirName = ".vex"
const modeLocal = "local"
const modeRemote = "remote"

type vexdConfig struct {
	rootVexPath string
}

func loadConfig() (vexdConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return vexdConfig{}, fmt.Errorf("resolve user home: %w", err)
	}
	return vexdConfig{rootVexPath: home}, nil
}

func buildRunCommand(args cli.RunArgs) (*cli.RunCommand, error) {
	if args.Mode != modeRemote && args.Mode != modeLocal {
		return nil, fmt.Errorf("vexd run: --mode %q inválido: debe ser \"remote\" o \"local\"", args.Mode)
	}

	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	if args.Mode == modeLocal {
		if err := linkVexHome(cfg.rootVexPath); err != nil {
			return nil, err
		}
	}

	projectsBasePath := filepath.Join(cfg.rootVexPath, vexHomeDirName, "projects")

	// --- Infrastructure: pipeline ---
	// Modo local: en lugar de clonar, crea un symlink hacia /appProject (volumen
	// montado por el CLI con el CWD del host). Modo remoto: clonación git normal.
	var projectClonerRepo pipDom.ProjectClonerRepository
	if args.Mode == modeLocal {
		projectClonerRepo = pippInfra.NewLocalProjectClonerRepository(projectsBasePath)
	} else {
		projectClonerRepo = pippInfra.NewProjectClonerRepository(projectsBasePath)
	}
	pipelineClonerRepo := pippInfra.NewPipelineClonerRepository(projectsBasePath)
	pipelineEnvRepo := pippInfra.NewPipelineEnvironmentRepository()
	pipelineStepRepo := pippInfra.NewPipelineStepRepository()
	pipelineWorkdirRepo := pippInfra.NewPipelineWorkdirRepository(projectsBasePath)
	projectTagRepo := pippInfra.NewProjectTagRepository()
	projectFingerprint := pippInfra.NewProjectFingerprint()

	// --- Infrastructure: step (vars, commands) ---
	varsStoreRepo := stepInfra.NewFileVarsStoreRepository(projectsBasePath)
	pipelineVarsRepo := stepInfra.NewPipelineVarsRepository()
	pipelineCommandRepo := stepInfra.NewPipelineCommandRepository()

	// --- Infrastructure: step status (local o remoto según flag) ---
	var (
		instStatusRepo stepStat.InstructionsStatusRepository
		varsStatusRepo stepStat.VariablesStatusRepository
		codeStatusRepo stepStat.CodeStatusRepository
		timeStatusRepo stepStat.TimeStatusRepository
		statusRepo     stepStat.StatusRepository
	)

	if args.Mode != modeLocal {
		codeStatusRepo = stepStatInfra.NewSupabaseCodeStatusRepository(
			args.StepCodeEndpoint, args.LogToken, args.ExecutionID,
		)
		instStatusRepo = stepStatInfra.NewSupabaseInstStatusRepository(
			args.StepInstEndpoint, args.LogToken, args.ExecutionID,
		)
		timeStatusRepo = stepStatInfra.NewSupabaseTimeStatusRepository(
			args.StepTimeEndpoint, args.LogToken, args.ExecutionID,
		)
		varsStatusRepo = stepStatInfra.NewSupabaseVarsStatusRepository(
			args.StepVarsEndpoint, args.LogToken, args.ExecutionID,
		)
		statusRepo = stepStatInfra.NewSupabaseStatusRepository(
			args.StepDeleteEndpoint, args.LogToken, args.ExecutionID,
		)
	} else {
		// Modo local: repos de archivo en disco.
		instStatusRepo = stepStatInfra.NewFileInstStatusRepository(projectsBasePath)
		varsStatusRepo = stepStatInfra.NewFileVarsStatusRepository(projectsBasePath)
		codeStatusRepo = stepStatInfra.NewFileCodeStatusRepository(projectsBasePath)
		timeStatusRepo = stepStatInfra.NewFileTimeStatusRepository(projectsBasePath)
		statusRepo = stepStatInfra.NewFileStatusRepository(varsStatusRepo, timeStatusRepo, instStatusRepo, codeStatusRepo)
	}

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

func linkVexHome(homeDir string) error {
	vexDir := filepath.Join(homeDir, vexHomeDirName)

	if err := os.RemoveAll(vexDir); err != nil {
		return fmt.Errorf("link vex home: eliminar %q: %w", vexDir, err)
	}

	if err := os.Symlink(vexHomeMountPoint, vexDir); err != nil {
		return fmt.Errorf("link vex home: crear symlink %q → %q: %w", vexDir, vexHomeMountPoint, err)
	}

	return nil
}
