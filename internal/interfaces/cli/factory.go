package cli

import (
	"fmt"
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
)

const (
	VexHomeDirName = ".vex"
	ModeLocal      = "local"
	ModeRemote     = "remote"

	// DefaultLocalProjectPath es el punto de montaje donde el CLI `vex` deja el
	// CWD del host cuando corre el motor dentro del contenedor (modo local).
	DefaultLocalProjectPath = "/appProject"
)

// EngineConfig son las rutas del sistema de archivos con las que se cablea el
// motor. Antes se derivaban dentro del factory (`os.UserHomeDir()` y una
// constante de montaje); ahora se las dicta el caller. Es lo que permite
// ejecutar el motor completo contra un directorio temporal en un test sin
// tocar el $HOME real de quien lo corre.
type EngineConfig struct {
	// RootVexPath es la raíz bajo la que vive el directorio ".vex".
	// En producción es $HOME.
	RootVexPath string

	// LocalProjectPath es el punto de montaje del proyecto en modo local.
	// Vacío significa DefaultLocalProjectPath.
	LocalProjectPath string
}

// ValidateMode rechaza cualquier modo que no sea "remote" o "local".
func ValidateMode(mode string) error {
	if mode != ModeRemote && mode != ModeLocal {
		return fmt.Errorf("vexd run: --mode %q inválido: debe ser \"remote\" o \"local\"", mode)
	}
	return nil
}

// BuildRunCommand ensambla las tres cadenas de responsabilidad, la policy y los
// repositorios (de archivo o de Supabase según `args.Mode`) y devuelve el
// RunCommand listo para ejecutar.
//
// No toca el $HOME del proceso: el enlace de `$HOME/.vex` hacia el volumen
// montado (`linkVexHome`) es responsabilidad del binario, no del cableado.
func BuildRunCommand(cfg EngineConfig, args RunArgs) (*RunCommand, error) {
	if err := ValidateMode(args.Mode); err != nil {
		return nil, err
	}
	if cfg.RootVexPath == "" {
		return nil, fmt.Errorf("vexd run: raíz de almacenamiento vacía")
	}
	localProjectPath := cfg.LocalProjectPath
	if localProjectPath == "" {
		localProjectPath = DefaultLocalProjectPath
	}

	projectsBasePath := filepath.Join(cfg.RootVexPath, VexHomeDirName, "projects")
	pipelinesBasePath := filepath.Join(cfg.RootVexPath, VexHomeDirName, "pipelines")

	// --- Infrastructure: pipeline ---
	// Modo local: en lugar de clonar, crea un symlink hacia el punto de montaje
	// del proyecto (el CWD del host). Modo remoto: clonación git normal.
	var projectClonerRepo pipDom.ProjectClonerRepository
	if args.Mode == ModeLocal {
		projectClonerRepo = pippInfra.NewLocalProjectClonerRepository(projectsBasePath, localProjectPath)
	} else {
		projectClonerRepo = pippInfra.NewProjectClonerRepository(projectsBasePath)
	}
	pipelineClonerRepo := pippInfra.NewPipelineClonerRepository(pipelinesBasePath)
	pipelineEnvRepo := pippInfra.NewPipelineEnvironmentRepository()
	pipelineStepRepo := pippInfra.NewPipelineStepRepository()
	pipelineStructureValidator := pipDom.NewPipelineStructureValidator()
	pipelineWorkdirRepo := pippInfra.NewPipelineWorkdirRepository(projectsBasePath)
	projectTagRepo := pippInfra.NewProjectTagRepository()
	projectFingerprint := pippInfra.NewProjectFingerprint()

	var varsStoreRepo stepDom.VarsStoreRepository
	if args.Mode != ModeLocal {
		varsStoreRepo = stepInfra.NewSupabaseVarsStoreRepository(
			args.StepStoreVarsEndpoint, args.LogToken, args.ExecutionID,
		)
	} else {
		varsStoreRepo = stepInfra.NewFileVarsStoreRepository(projectsBasePath)
	}
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

	if args.Mode != ModeLocal {
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
		pipDom.NewStepsLoaderHandler(pipelineStepRepo, pipelineStructureValidator),
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
	// El orden importa: los dos handlers de almacén cargan ANTES que las
	// variables declaradas por el pipelinecode, de modo que lo declarado gana
	// sobre lo almacenado.
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

	return NewRunCommand(createExec), nil
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
