package cli

import (
	"fmt"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	pipDom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	stepDom "github.com/jairoprogramador/vex-engine/internal/domain/step"
	cacheInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/cache"
	cmdInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/command"
	pippInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	sharedInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/shared"
	stepInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/step"
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

	// El caché NO cuelga de `projects/`: está direccionado por contenido, y el
	// proyecto es una de las siete dimensiones que van dentro del hash, no un
	// tramo de la ruta (spec 10 §5.1). Que el directorio sea aparte hace además
	// visible su regla de vida: se puede borrar entero sin consecuencias, cosa
	// que `projects/` —donde vive el almacén de variables— no admite.
	cacheBasePath := filepath.Join(cfg.RootVexPath, VexHomeDirName, "cache")

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
	contentFingerprint := pippInfra.NewContentFingerprint()

	// El reloj del proceso es la ÚNICA fuente de instantes del dominio: la usan
	// el agregado (startedAt/finishedAt) y el cálculo de versión (spec 07 §5.1).
	clock := sharedInfra.NewSystemClock()

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

	// --- Infrastructure: caché de re-ejecución ---
	//
	// UN repositorio, y sin rama por modo. Aquí había cuatro repositorios × dos
	// implementaciones —archivo y Supabase— seleccionados por `args.Mode`; los
	// cuatro de Supabase se borran con la spec 10, lo que deja las edge functions
	// `status-*` sin cliente (su retirada va con la spec 26) y con los cuatro se
	// van los `--step-{code,inst,time,vars}-endpoint`, que ya no lee nadie.
	//
	// Consecuencia declarada: **el modo remoto pierde caché desde esta spec**,
	// no desde la 16. La máquina de Fly es efímera, así que este repositorio
	// arranca frío en cada ejecución. Es una degradación de RENDIMIENTO, no de
	// correctitud: un caché frío ejecuta de más, nunca de menos. Quien devuelve
	// el caché compartido es la spec 16, con un destino de estado explícito.
	entries := cacheInfra.NewFileEntriesRepository(cacheBasePath)

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
		pipDom.NewVersionCalculatorHandler(projectTagRepo, clock),
		pipDom.NewInitVarsHandler(),
		pipDom.NewProjectStatusHandler(contentFingerprint),
		pipDom.NewPipelineRunnerHandler(),
	)
	executablePipeline := pipDom.NewPipelineExecutable(pipelineHead)

	// --- Domain: step handler chain ---
	//
	// Aquí se construía el `RuleRegistry` con las cuatro reglas y el
	// `PolicyBuilder` que las elegía por nombre de paso. Los tres tipos
	// desaparecieron con la spec 10: la decisión es «¿existe esta clave?», y para
	// eso no hay nada que registrar ni que componer.
	// El orden importa: los dos handlers de almacén cargan ANTES que las
	// variables declaradas por el pipelinecode, de modo que lo declarado gana
	// sobre lo almacenado.
	stepHead := chainStepHandlers(
		stepDom.NewVarsStoreSharedHandler(varsStoreRepo),
		stepDom.NewVarsStoreStepHandler(varsStoreRepo),
		stepDom.NewVarsHandler(pipelineVarsRepo),
		stepDom.NewStepRunnerHandler(pipelineCommandRepo, entries),
	)
	executableStep := stepDom.NewStepExecutable(stepHead, varsStoreRepo, entries)

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
		clock,
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
