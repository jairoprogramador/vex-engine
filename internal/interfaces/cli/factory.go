package cli

import (
	"fmt"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	pipDom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	stateDom "github.com/jairoprogramador/vex-engine/internal/domain/state"
	stepDom "github.com/jairoprogramador/vex-engine/internal/domain/step"
	cacheInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/cache"
	cmdInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/command"
	pippInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	sharedInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/shared"
	stateInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/state"
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

	// Las dos tiendas, y los dos directorios existen separados para que sus
	// reglas de vida se vean desde `ls`:
	//
	//   state/  es la VERDAD. Un registro por ejecución real de un step, nunca
	//           sobrescrito, con los identificadores de recursos que existen de
	//           verdad en la nube. No se borra nunca (spec 11).
	//   cache/  es el ÍNDICE. Derivable, desechable, y no participa en ninguna
	//           decisión: `rm -rf` sobre él no cambia lo que el motor decide.
	//
	// Ninguno cuelga de `projects/`: el índice está direccionado por contenido y
	// el proyecto va dentro del hash, y el almacén de registros lo lleva como
	// primer tramo pero con su propio esquema de rutas.
	stateBasePath := filepath.Join(cfg.RootVexPath, VexHomeDirName, "state")
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

	// El lector de `steps/NN-x/config.yaml` tiene UN dueño y dos consumidores
	// (spec 13): el validador de estructura, que exige antes del primer step que
	// un `config.yaml` presente declare un ámbito del vocabulario cerrado; y el
	// handler 03 de la cadena de step, que necesita el ámbito para saber dónde
	// consultar y dónde escribir. Dos lecturas de un archivo diminuto que ya está
	// en disco local, a cambio de que la regla de vocabulario esté escrita una
	// sola vez.
	pipelineStepConfigRepo := stepInfra.NewPipelineStepConfigRepository()

	// El manifiesto de la raíz (`vexpipeline.yaml`) y los dos lectores del step
	// entran al validador porque las dos reglas de la spec 14 son de CARGA: que un
	// `resolve` exija `schema_version: 2`, y que el `from`/`key` de un
	// `step-output` apunten a un step anterior que de verdad declara ese output.
	// Las dos mueven un fallo que hoy ocurre a mitad del despliegue —con `test` y
	// `supply` ya ejecutados— al momento en que fallar no deja efectos a medias.
	pipelineManifestRepo := pippInfra.NewPipelineManifestRepository()
	pipelineVarsRepo := stepInfra.NewPipelineVarsRepository()
	pipelineCommandRepo := stepInfra.NewPipelineCommandRepository()

	pipelineStructureValidator := pipDom.NewPipelineStructureValidator(
		pipelineStepConfigRepo, pipelineManifestRepo, pipelineVarsRepo, pipelineCommandRepo)
	pipelineWorkdirRepo := pippInfra.NewPipelineWorkdirRepository(projectsBasePath)
	projectTagRepo := pippInfra.NewProjectTagRepository()
	contentFingerprint := pippInfra.NewContentFingerprint()

	// El reloj del proceso es la ÚNICA fuente de instantes del dominio: la usan
	// el agregado (startedAt/finishedAt) y el cálculo de versión (spec 07 §5.1).
	clock := sharedInfra.NewSystemClock()

	// --- Infrastructure: almacén de registros de step (spec 11) ---
	//
	// El adaptador de Supabase sobrevive al cambio de puerto, y no de milagro: en
	// modo remoto la máquina de Fly es efímera, así que un almacén de archivo no
	// guardaría nada y un ARN extraído allí se perdería en cada ejecución —que es
	// exactamente el daño que esta spec existe para impedir—. Lo que NO puede
	// hacer es historia: la edge function guarda el último conjunto por (ámbito,
	// step), así que sus registros vienen sin atribuir y ningún step revive en
	// remoto. Es el mismo comportamiento que desde la spec 10, no uno nuevo. Se
	// retira en la spec 16.
	var records stateDom.Records
	if args.Mode != ModeLocal {
		records = stateInfra.NewSupabaseRecordsRepository(
			args.StepStoreVarsEndpoint, args.LogToken, args.ExecutionID,
		)
	} else {
		records = stateInfra.NewFileRecordsRepository(stateBasePath)
	}
	recordIDs := stateInfra.NewULIDRecordIDFactory()

	// --- Infrastructure: índice de contenido → registro ---
	//
	// De archivo y sin rama por modo, como desde la spec 10. Que en remoto
	// arranque frío daba igual entonces —el caché es una optimización— y da más
	// igual ahora: desde la spec 11 este índice no decide nada, así que estar
	// vacío no cambia ninguna decisión ni en local ni en remoto.
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
	// El orden DEJÓ DE DECIDIR QUIÉN GANA (spec 12 §5.2'): la precedencia de
	// variables vive en `command.ExecutionVariableMap.Add`, sobre el enum ordenado
	// `command.Origin`, y quien intercambie dos de estos handlers no cambia qué
	// valor prevalece. Hasta la spec 12 sí lo cambiaba: el almacén cargaba primero
	// para que lo declarado, al escribir después, lo pisara.
	//
	// Pero el orden NO es indiferente, y la spec 12 §5.3 —que mandaba mover el
	// handler de las declaradas delante del almacén— se retira por eso (§10, H1).
	// Ese handler no solo AÑADE variables: las RESUELVE, interpolando `${var.…}`
	// contra el mapa acumulado tal como esté en ese instante. Cargarlo primero deja
	// fuera de su vista el registro del propio step, y un literal declarado que
	// interpole un nombre que solo vive ahí falla con «variable faltante». El
	// resultado de la precedencia es el mismo en los dos órdenes, así que moverlo
	// no compraba nada y costaba eso.
	//
	// Por tanto: se carga de menor a mayor COMPLETITUD del mapa —almacén primero,
	// declaradas después—, y quien gana lo dice `Origin`.
	//
	// Y el handler 03 recibe `records`, no `entries`: la decisión de re-ejecutar
	// lee el último registro de la clave de posición y NO consulta el índice
	// (spec 11 §5.5). Que el índice no llegue hasta aquí es lo que impide que
	// vuelva a ser una tienda con estado.
	//
	// Son TRES handlers desde la spec 13, no cuatro: los dos del almacén se
	// colapsan en uno que lee los dos ámbitos —el de proyecto y el del ambiente—,
	// porque el ámbito declarado decide dónde se ESCRIBE y no qué se puede leer
	// (§5.4). Los archivos se renumeraron con ellos: el número dice la posición.
	// El handler 02 recibe además los RESOLUTORES de declaraciones (spec 14
	// §5.3'): uno por valor de `resolve`, elegidos por el vocabulario cerrado. El
	// de `state` necesita `records` porque lee el registro del propio step bajo el
	// ámbito declarado; el de `step-output` no necesita nada, porque lee del mapa
	// acumulado — que es lo que el mapa acumulado pasa a ser con esta spec: una
	// caché de resolución, no el modelo.
	stepHead := chainStepHandlers(
		stepDom.NewVarsStoreHandler(records),
		stepDom.NewVarsHandler(pipelineVarsRepo, stepDom.NewDeclarationResolvers(records)),
		stepDom.NewStepRunnerHandler(pipelineCommandRepo, pipelineStepConfigRepo, records),
	)
	executableStep := stepDom.NewStepExecutable(stepHead, records, recordIDs, entries)

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
