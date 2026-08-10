package cli

import (
	"fmt"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	pipDom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/domain/record"
	stepDom "github.com/jairoprogramador/vex-engine/internal/domain/step"
	cmdInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/command"
	deploymentInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/deployment"
	notifyInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/notify"
	pippInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/pipeline"
	recordInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/record"
	sharedInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/shared"
	stateInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/state"
	stepInfra "github.com/jairoprogramador/vex-engine/internal/infrastructure/step"
)

const VexHomeDirName = ".vex"

// Las dos tiendas del registro que viven en el ÁREA DE TRABAJO del motor, no en
// el destino: nadie las lee durante la ejecución, así que se bufferizan aquí y
// las empuja la spec 21. La cabeza del linaje, que sí se lee antes de decidir,
// cuelga del destino (ver `newStateStores`).
const (
	objectsDirName = "objects"
	eventsDirName  = "events"
)

// EngineConfig son las rutas del sistema de archivos con las que se cablea el
// motor. Antes se derivaban dentro del factory (`os.UserHomeDir()` y una
// constante de montaje); ahora se las dicta el caller. Es lo que permite
// ejecutar el motor completo contra un directorio temporal en un test sin
// tocar el $HOME real de quien lo corre.
//
// Los dos campos son lo que queda del enum `--mode` (spec 16 §5.4). Ninguno
// tiene ya una constante detrás: `/appProject` era el punto de montaje que el
// CLI `vex` usaba en modo local y pasa a ser lo que el caller diga.
type EngineConfig struct {
	// RootVexPath es la raíz bajo la que vive el directorio ".vex" —los clones
	// del proyecto y del pipelinecode, y los workdirs—. En producción es $HOME
	// o el volumen que el invocador monte.
	//
	// El ESTADO ya no cuelga de aquí: vive en el destino configurado.
	RootVexPath string

	// LocalProjectPath es el punto de montaje del proyecto ya presente en disco.
	// Vacío significa que el proyecto se CLONA, que es la otra mitad de lo que
	// `--mode` decidía.
	LocalProjectPath string
}

// BuildRunCommand ensambla las tres cadenas de responsabilidad y los
// repositorios —el almacén y el índice según el DESTINO configurado— y devuelve
// el RunCommand listo para ejecutar.
//
// No toca el $HOME del proceso: desde la spec 16 nadie lo reescribe.
func BuildRunCommand(cfg EngineConfig, args RunArgs) (*RunCommand, error) {
	if cfg.RootVexPath == "" {
		return nil, fmt.Errorf("vexd run: raíz de almacenamiento vacía")
	}

	// El destino del estado es CONFIGURACIÓN, no un modo, y no tiene default en
	// ninguna capa: si falta, el motor no arranca (spec 16 §5.1). El default
	// vive en quien invoca.
	destino, err := readStateConfig(args)
	if err != nil {
		return nil, err
	}
	stores, err := newStateStores(destino)
	if err != nil {
		return nil, err
	}

	// Y el área de trabajo propia, que sí es dominio del motor y sí tiene
	// default. Desde la spec 18 ya no está vacía: ahí escriben el objeto de
	// despliegue y los hechos del intento, que es lo que hace que «registrar es
	// incondicional» sea una propiedad observable y no una intención.
	stagingPath, err := resolveStagingDir(args.StagingDir, cfg.RootVexPath)
	if err != nil {
		return nil, err
	}

	projectsBasePath := filepath.Join(cfg.RootVexPath, VexHomeDirName, "projects")
	pipelinesBasePath := filepath.Join(cfg.RootVexPath, VexHomeDirName, "pipelines")

	// El reloj del proceso es la ÚNICA fuente de instantes del dominio: la usan
	// el agregado (startedAt/finishedAt), el cálculo de versión (spec 07 §5.1),
	// la ventana de reutilización del clon y el sobre de cada hecho (spec 18).
	clock := sharedInfra.NewSystemClock()

	// --- Infrastructure: pipeline ---
	// Con el proyecto ya en disco (el CWD del host montado en el contenedor), en
	// lugar de clonar se crea un symlink hacia su punto de montaje. Sin él,
	// clonación git normal. Lo dice el caller, no un enum.
	var projectClonerRepo pipDom.ProjectClonerRepository
	if cfg.LocalProjectPath != "" {
		projectClonerRepo = pippInfra.NewLocalProjectClonerRepository(projectsBasePath, cfg.LocalProjectPath)
	} else {
		projectClonerRepo = pippInfra.NewProjectClonerRepository(projectsBasePath)
	}
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

	// El clonador del pipelinecode necesita el manifiesto y el reloj desde la
	// spec 18: la ventana de reutilización la declara el pipelinecode y la edad
	// del clon se mide contra la marca que él mismo deja (§5.4).
	pipelineClonerRepo := pippInfra.NewPipelineClonerRepository(
		pipelinesBasePath, pipelineManifestRepo, clock)

	// --- Infrastructure: almacén de registros de step e índice (specs 11 y 16) ---
	//
	// Los dos salen del DESTINO, y salen juntos: son una familia coherente y
	// mezclarlas no debe ser construible (`newStateStores`). Aquí desaparece el
	// adaptador de Supabase, que era la otra mitad de `--mode`: guardaba el
	// último conjunto por (ámbito, step) sin historia, así que devolvía registros
	// SIN ATRIBUIR y en modo remoto ningún step revivía —ni por huella, porque no
	// había ninguna que comparar, ni por `max_age`, porque la edad se medía contra
	// el instante cero—. Un `max_age` declarado en el pipelinecode se ignoraba en
	// silencio; con el destino explícito las dos reglas significan lo mismo en
	// todas partes, sin código nuevo.
	records := stores.records
	entries := stores.entries
	recordIDs := stateInfra.NewULIDRecordIDFactory()

	// --- Infrastructure: command (shell, filesystem) ---
	fileSystem := cmdInfra.NewFileSystemManager()
	shellRunner := cmdInfra.NewShellCommandRunner()

	// --- Infrastructure: registro de despliegue (specs 17 y 18) ---
	//
	// Las tres tiendas NO cuelgan del mismo sitio, y la diferencia no es de gusto
	// sino de quién las lee (spec 21 §5.1):
	//
	//	objects/  events/   →  área de trabajo. Nadie las lee durante la ejecución,
	//	                       así que se bufferizan y las empuja la spec 21.
	//	lineage/            →  DESTINO. Hay que leer la cabeza ANTES de ejecutar:
	//	                       sin `parent` no hay posición, y una historia que
	//	                       empieza vacía en cada máquina efímera derivaría dos
	//	                       veces el mismo `deployment_id`.
	objectStore := deploymentInfra.NewFileObjectStore(filepath.Join(stagingPath, objectsDirName))
	lineageStore := stores.lineages

	// R-7 hecho cableado: el registro es la FUENTE y las líneas para humanos se
	// derivan de él (spec 19 §5.5). El renderizador se interpone entre el emisor y
	// el archivo —decorador del sink, no un bus— así que los hechos se escriben
	// igual aunque nadie esté mirando: registrar es incondicional, narrar no.
	eventRenderer := notifyInfra.NewEventRenderer(
		recordInfra.NewJSONLEventSink(filepath.Join(stagingPath, eventsDirName)))
	emitter := record.NewEmitter(clock, recordInfra.NewUUIDv7EventIDFactory(), eventRenderer)

	// El adaptador de los dos puertos de hechos. Es UNO y no dos porque los tres
	// hechos que traduce salen del mismo emisor y con la misma numeración: `seq`
	// es la posición dentro del INTENTO, no dentro de una cadena.
	facts := record.NewFacts(emitter)

	// El material del pipelinecode se lee UNA vez, en la cadena de pipeline, y la
	// de step lo consume (spec 18 §5.2). El objeto lo comparten los dos lados
	// porque no hay otro canal: el `ExecutionContext` vive en `command`, que no
	// puede importar `step` sin un ciclo.
	loadedPipelinecode := stepDom.NewLoadedPipelinecode()

	// --- Domain: pipeline handler chain (orden 01 → 10) ---
	//
	// El eslabón nuevo es el 09, y con él la cadena deja de ser «prepara y
	// ejecuta» para ser «declara qué vas a hacer y luego hazlo». Va en la 09 y no
	// antes porque la identidad se calcula sobre la fuente REALMENTE USADA:
	// necesita que el clonador ya haya decidido si trae el pipelinecode o
	// reutiliza el que hay (spec 18 §5.4).
	pipelineHead := chainPipelineHandlers(
		pipDom.NewProjectClonerHandler(projectClonerRepo),
		pipDom.NewPipelineClonerHandler(pipelineClonerRepo, emitter),
		pipDom.NewEnvironmentLoaderHandler(pipelineEnvRepo),
		pipDom.NewStepsLoaderHandler(pipelineStepRepo, pipelineStructureValidator),
		pipDom.NewCopyWorkdirHandler(pipelineWorkdirRepo),
		pipDom.NewVersionCalculatorHandler(projectTagRepo, clock),
		pipDom.NewInitVarsHandler(),
		pipDom.NewProjectStatusHandler(contentFingerprint),
		pipDom.NewDeploymentResolverHandler(
			pipelineCommandRepo,
			pipelineStepConfigRepo,
			pipelineVarsRepo,
			pipelineManifestRepo,
			contentFingerprint,
			loadedPipelinecode,
			objectStore,
			lineageStore,
			emitter,
		),
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
		stepDom.NewVarsHandler(loadedPipelinecode, stepDom.NewDeclarationResolvers(records), facts),
		stepDom.NewStepRunnerHandler(loadedPipelinecode, records),
	)
	executableStep := stepDom.NewStepExecutable(stepHead, records, recordIDs, entries, facts)

	// --- Domain: command handler chain ---
	fileInterpolator := command.NewFileInterpolator(fileSystem)
	commandHead := chainCommandHandlers(
		command.NewFilesInterpolatorHandler(*fileInterpolator),
		command.NewCommandInterpolatorHandler(),
		command.NewCommandRunnerHandler(shellRunner),
		command.NewRegexCheckerHandler(),
		command.NewVarsExtractorHandler(facts),
	)
	executableCommand := command.NewCommandExecutable(commandHead, facts)

	// --- Application ---
	createExec := usecase.NewCreateExecutionUseCase(
		executablePipeline,
		executableCommand,
		executableStep,
		clock,
		emitter,
	)

	return NewRunCommand(createExec, destino, stagingPath, eventRenderer), nil
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
