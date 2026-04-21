package application

import (
	"context"
	"fmt"
	"sync"

	exeAgg "github.com/jairoprogramador/vex-engine/internal/domain/execution/aggregates"
	exePrt "github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
	exeVos "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
	pipDom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	pipPrt "github.com/jairoprogramador/vex-engine/internal/domain/pipeline/ports"
	pipSer "github.com/jairoprogramador/vex-engine/internal/domain/pipeline/services"
	projDomain "github.com/jairoprogramador/vex-engine/internal/domain/project"
	proPorts "github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
	proServices "github.com/jairoprogramador/vex-engine/internal/domain/project/services"
	storagePorts "github.com/jairoprogramador/vex-engine/internal/domain/storage/ports"
	storageSvc "github.com/jairoprogramador/vex-engine/internal/domain/storage/services"
	storageVos "github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
	worAgg "github.com/jairoprogramador/vex-engine/internal/domain/workspace/aggregates"
	gitInf "github.com/jairoprogramador/vex-engine/internal/infrastructure/git"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
)

// ExecutionOrchestrator coordina todo el ciclo de vida de una ejecución de pipeline:
// crea el agregado Execution, lo persiste, lanza la goroutine de ejecución y retorna
// el ID inmediatamente al caller (modelo no bloqueante para el daemon HTTP).
type ExecutionOrchestrator struct {
	rootVexPath     string
	projectSvc      *ProjectService
	workspaceSvc    *WorkspaceService
	gitCloner       gitInf.RepositoryGit
	versionResolver *proServices.VersionResolver
	projectFetcher  proPorts.RepositoryFetcher
	fetcher         pipPrt.RepositoryFetcher
	pipelineLoader  *pipSer.PlanResolver
	fingerprintSvc  storagePorts.FingerprintFilesystem
	decider         *storageSvc.ExecutionDecider
	stepExecutor    exePrt.StepExecutor
	copyWorkdir     exePrt.CopyWorkdir
	varsRepository  exePrt.VarsRepository
	emitter         exePrt.LogEmitter
	executionRepo   exePrt.ExecutionRepository
	// liveExecutions mantiene los agregados activos en memoria para poder cancelarlos.
	liveExecutions sync.Map // map[string]*exeAgg.Execution
}

func NewExecutionOrchestrator(
	rootVexPath string,
	projectSvc *ProjectService,
	workspaceSvc *WorkspaceService,
	gitCloner gitInf.RepositoryGit,
	versionResolver *proServices.VersionResolver,
	projectFetcher proPorts.RepositoryFetcher,
	fetcher pipPrt.RepositoryFetcher,
	pipelineLoader *pipSer.PlanResolver,
	fingerprintSvc storagePorts.FingerprintFilesystem,
	decider *storageSvc.ExecutionDecider,
	stepExecutor exePrt.StepExecutor,
	copyWorkdir exePrt.CopyWorkdir,
	varsRepository exePrt.VarsRepository,
	emitter exePrt.LogEmitter,
	executionRepo exePrt.ExecutionRepository,
) *ExecutionOrchestrator {
	return &ExecutionOrchestrator{
		rootVexPath:     rootVexPath,
		projectSvc:      projectSvc,
		workspaceSvc:    workspaceSvc,
		gitCloner:       gitCloner,
		versionResolver: versionResolver,
		projectFetcher:  projectFetcher,
		fetcher:         fetcher,
		pipelineLoader:  pipelineLoader,
		fingerprintSvc:  fingerprintSvc,
		decider:         decider,
		stepExecutor:    stepExecutor,
		copyWorkdir:     copyWorkdir,
		varsRepository:  varsRepository,
		emitter:         emitter,
		executionRepo:   executionRepo,
	}
}

// Run crea el agregado Execution, lo persiste y lanza la goroutine de ejecución.
// Retorna el ExecutionID inmediatamente — el caller puede usarlo para polling
// o abrir un stream de logs via SSE.
func (o *ExecutionOrchestrator) Run(ctx context.Context, cmd dto.RequestInput) (exeVos.ExecutionID, error) {
	runtimeCfg := exeVos.NewRuntimeConfig(cmd.Execution.RuntimeImage, cmd.Execution.RuntimeTag)

	execution := exeAgg.NewExecution(
		cmd.Project.ID,
		cmd.Project.Name,
		cmd.Pipeline.URL,
		cmd.Pipeline.Ref,
		cmd.Execution.Step,
		cmd.Execution.Environment,
		runtimeCfg,
	)

	if err := o.executionRepo.Save(ctx, execution); err != nil {
		return exeVos.ExecutionID{}, fmt.Errorf("execution orchestrator: guardar ejecución inicial: %w", err)
	}

	childCtx, cancelFn := context.WithCancel(context.Background())
	execution.SetCancelFn(cancelFn)

	o.liveExecutions.Store(execution.ID().String(), execution)

	go o.executePlan(childCtx, execution.ID(), cmd)

	return execution.ID(), nil
}

// Cancel interrumpe una ejecución en curso invocando su cancelFn y actualizando
// el estado en el repositorio a StatusCancelled.
func (o *ExecutionOrchestrator) Cancel(ctx context.Context, executionID exeVos.ExecutionID) error {
	val, ok := o.liveExecutions.Load(executionID.String())
	if !ok {
		return fmt.Errorf("execution orchestrator: cancel: execution %s not found in memory", executionID.String())
	}

	execution, ok := val.(*exeAgg.Execution)
	if !ok {
		return fmt.Errorf("execution orchestrator: cancel: unexpected type in live executions map")
	}

	execution.Cancel()
	o.liveExecutions.Delete(executionID.String())

	if err := o.executionRepo.UpdateStatus(ctx, executionID, exeVos.StatusCancelled); err != nil {
		return fmt.Errorf("execution orchestrator: cancel: update status: %w", err)
	}

	return nil
}

// executePlan corre en goroutine y ejecuta el pipeline completo.
func (o *ExecutionOrchestrator) executePlan(ctx context.Context, executionID exeVos.ExecutionID, cmd dto.RequestInput) {
	defer o.liveExecutions.Delete(executionID.String())

	emit := func(line string) {
		o.emitter.Emit(executionID, line)
	}

	if err := o.executionRepo.UpdateStatus(ctx, executionID, exeVos.StatusRunning); err != nil {
		emit(fmt.Sprintf("ERROR: no se pudo actualizar el estado a running: %v", err))
	}

	if err := o.runPipeline(ctx, executionID, cmd); err != nil {
		emit(fmt.Sprintf("ERROR: %v", err))
		_ = o.executionRepo.UpdateStatus(ctx, executionID, exeVos.StatusFailed)
		return
	}

	if err := o.executionRepo.UpdateStatus(ctx, executionID, exeVos.StatusSucceeded); err != nil {
		emit(fmt.Sprintf("ADVERTENCIA: ejecución exitosa pero no se pudo actualizar el estado: %v", err))
	}
	emit("Ejecución completada con éxito.")
}

// runPipeline contiene la lógica central: carga proyecto, workspace, clona template,
// construye plan y ejecuta cada paso usando el ExecutionDecider para decidir skip/run.
func (o *ExecutionOrchestrator) runPipeline(ctx context.Context, executionID exeVos.ExecutionID, cmd dto.RequestInput) error {
	emit := func(line string) {
		o.emitter.Emit(executionID, line)
	}

	proj, err := o.projectSvc.FromDTO(cmd)
	if err != nil {
		return fmt.Errorf("construir proyecto: %w", err)
	}

	version, commitHash, projectPath, err := o.versionResolver.NextVersion(ctx, proj.URL(), proj.Ref())
	if err != nil {
		return fmt.Errorf("calcular versión: %w", err)
	}

	workspace, err := o.loadWorkspace(proj, cmd)
	if err != nil {
		return err
	}

	if err := o.cloneTemplate(ctx, cmd, workspace.TemplatePath()); err != nil {
		return err
	}

	planDef, err := o.fetchAndBuildPlan(ctx, cmd)
	if err != nil {
		return err
	}

	environment := planDef.Environment().Value()

	projectVars, err := o.prepareProjectVariables(proj)
	if err != nil {
		return fmt.Errorf("preparar variables del proyecto: %w", err)
	}

	othersVars, err := o.prepareOthersVariables(environment, projectPath, version.String(), commitHash)
	if err != nil {
		return fmt.Errorf("preparar variables de entorno: %w", err)
	}

	cumulativeVars := make(exeVos.VariableSet)
	cumulativeVars.AddAll(projectVars)
	cumulativeVars.AddAll(othersVars)

	// templateName se extrae de la URL del pipeline (último segmento)
	pipelineURL, err := pipDom.NewPipelineURL(cmd.Pipeline.URL)
	if err != nil {
		return fmt.Errorf("url de pipeline inválida: %w", err)
	}
	templateName := pipelineURL.Name()

	emit("Iniciando la ejecución del plan...")
	emit(fmt.Sprintf("  - Entorno: %s", environment))
	emit(fmt.Sprintf("  - Versión: %s", version.String()))
	emit(fmt.Sprintf("  - Commit: %s", commitHash))

	for _, stepDef := range planDef.Steps() {
		emit(fmt.Sprintf("Ejecutando paso %s ...", stepDef.Name().Name()))

		fingerprints, err := o.generateStepFingerprints(projectPath, environment, workspace, stepDef.Name())
		if err != nil {
			return fmt.Errorf("generar fingerprint para el paso '%s': %w", stepDef.Name().Name(), err)
		}

		stepName, err := storageVos.NewStepName(stepDef.Name().Name())
		if err != nil {
			return fmt.Errorf("nombre de paso inválido '%s': %w", stepDef.Name().Name(), err)
		}

		key := storageVos.NewStorageKey(proj.Name().String(), templateName, stepName)

		decision, err := o.decider.Decide(ctx, key, fingerprints)
		if err != nil {
			return fmt.Errorf("decidir ejecución del paso '%s': %w", stepDef.Name().Name(), err)
		}

		varsStepPath := workspace.VarsFilePath(environment, stepDef.Name().Name())
		varsStep, err := o.varsRepository.Get(varsStepPath)
		if err != nil {
			return fmt.Errorf("obtener variables del paso '%s' en entorno '%s': %w", stepDef.Name().Name(), environment, err)
		}
		cumulativeVars.AddAll(varsStep)

		varsSharedPath := workspace.VarsFilePath("shared", stepDef.Name().Name())
		varsShared, err := o.varsRepository.Get(varsSharedPath)
		if err != nil {
			return fmt.Errorf("obtener variables del paso '%s' en entorno 'shared': %w", stepDef.Name().Name(), err)
		}
		cumulativeVars.AddAll(varsShared)

		if !decision.ShouldRun() {
			emit(fmt.Sprintf("  - Paso '%s' sin cambios en el historial de ejecución (match de %s). Omitiendo.",
				stepDef.Name().Name(), decision.MatchedAt().Format("2006-01-02 15:04:05")))
			continue
		}

		envStepPath := workspace.ScopeWorkdirPath(planDef.Environment().Value(), stepDef.Name().Name())
		if err := o.copyWorkdir.Copy(ctx, workspace.StepTemplatePath(stepDef.Name().FullName()), envStepPath, false); err != nil {
			return fmt.Errorf("copiar paso '%s' al workspace de entorno: %w", stepDef.Name().Name(), err)
		}

		sharedStepPath := workspace.ScopeWorkdirPath(exeVos.SharedScope, stepDef.Name().Name())
		if err := o.copyWorkdir.Copy(ctx, workspace.StepTemplatePath(stepDef.Name().FullName()), sharedStepPath, true); err != nil {
			return fmt.Errorf("copiar paso '%s' al workspace compartido: %w", stepDef.Name().Name(), err)
		}

		execStep, err := mapToExecutionStep(stepDef, envStepPath, sharedStepPath)
		if err != nil {
			return fmt.Errorf("mapear definición del paso '%s': %w", stepDef.Name().Name(), err)
		}

		execResult, err := o.stepExecutor.Execute(ctx, execStep, cumulativeVars, o.emitter, executionID)
		if err != nil {
			return fmt.Errorf("ejecución del paso '%s': %w", stepDef.Name().Name(), err)
		}
		if execResult.Error != nil || execResult.Status == exeVos.Failure {
			emit("--- Logs del fallo ---")
			emit(execResult.Logs)
			emit("---------------------")
			return fmt.Errorf("el paso '%s' finalizó con error: %w", stepDef.Name().Name(), execResult.Error)
		}

		emit(fmt.Sprintf("  - Paso '%s' completado:\n%s", stepDef.Name().Name(), execResult.Logs))
		cumulativeVars.AddAll(execResult.OutputVars)

		outputSharedVars := execResult.OutputVars.Filter(func(v exeVos.OutputVar) bool {
			return v.IsShared()
		})
		if !outputSharedVars.Equals(varsShared) {
			if err := o.varsRepository.Save(varsSharedPath, outputSharedVars); err != nil {
				return fmt.Errorf("guardar variables compartidas del paso '%s': %w", stepDef.Name().Name(), err)
			}
		}

		outputStepVars := execResult.OutputVars.Filter(func(v exeVos.OutputVar) bool {
			return !v.IsShared()
		})
		if !outputStepVars.Equals(varsStep) {
			if err := o.varsRepository.Save(varsStepPath, outputStepVars); err != nil {
				return fmt.Errorf("guardar variables del paso '%s' en entorno '%s': %w", stepDef.Name().Name(), environment, err)
			}
		}

		if err := o.decider.RecordSuccess(ctx, key, fingerprints); err != nil {
			// El paso fue exitoso. No fallamos la ejecución por esto, pero lo notificamos.
			emit(fmt.Sprintf("ADVERTENCIA: no se pudo guardar el historial de ejecución del paso '%s'. Se re-ejecutará la próxima vez. Error: %v",
				stepDef.Name().Name(), err))
		}
	}

	if cmd.Execution.Step == "deploy" {
		if err := o.projectFetcher.CreateTagForCommit(ctx, projectPath, commitHash, version.String()); err != nil {
			emit(fmt.Sprintf("ADVERTENCIA: no se pudo crear el tag del commit. Error: %v", err))
		}
	}

	return nil
}

func (o *ExecutionOrchestrator) loadWorkspace(project *projDomain.Project, cmd dto.RequestInput) (*worAgg.Workspace, error) {
	pipelineURL, err := pipDom.NewPipelineURL(cmd.Pipeline.URL)
	if err != nil {
		return nil, fmt.Errorf("cargar workspace: url de pipeline inválida: %w", err)
	}
	templateDirName := pipelineURL.Name()

	workspace, err := o.workspaceSvc.NewWorkspace(
		o.rootVexPath, project.Name().String(), templateDirName)
	if err != nil {
		return nil, fmt.Errorf("cargar workspace: %w", err)
	}
	return workspace, nil
}

func (o *ExecutionOrchestrator) cloneTemplate(ctx context.Context, cmd dto.RequestInput, templateLocalPath string) error {
	if err := o.gitCloner.Clone(ctx, cmd.Pipeline.URL, cmd.Pipeline.Ref, templateLocalPath); err != nil {
		return fmt.Errorf("clonar repositorio de plantillas: %w", err)
	}
	return nil
}

func (o *ExecutionOrchestrator) fetchAndBuildPlan(ctx context.Context, cmd dto.RequestInput) (*pipDom.PipelinePlan, error) {
	repoURL, err := pipDom.NewPipelineURL(cmd.Pipeline.URL)
	if err != nil {
		return nil, fmt.Errorf("fetchAndBuildPlan: url inválida: %w", err)
	}

	ref, err := pipDom.NewPipelineRef(cmd.Pipeline.Ref)
	if err != nil {
		return nil, fmt.Errorf("fetchAndBuildPlan: ref inválida: %w", err)
	}

	localPath, err := o.fetcher.Fetch(ctx, repoURL, ref)
	if err != nil {
		return nil, fmt.Errorf("obtener repositorio pipeline: %w", err)
	}

	limit := pipDom.NewStepLimit(cmd.Execution.Step)
	plan, err := o.pipelineLoader.Load(ctx, localPath, cmd.Execution.Environment, limit)
	if err != nil {
		return nil, fmt.Errorf("cargar plan: %w", err)
	}
	return plan, nil
}

func (o *ExecutionOrchestrator) prepareProjectVariables(project *projDomain.Project) (exeVos.VariableSet, error) {
	idStr := project.ID().String()
	if len(idStr) > 8 {
		idStr = idStr[:8]
	}
	return exeVos.NewVariableSetFromMap(map[string]string{
		"project_id":           idStr,
		"project_name":         project.Name().String(),
		"project_organization": project.Org().String(),
		"project_team":         project.Team().String(),
	})
}

func (o *ExecutionOrchestrator) prepareOthersVariables(environment, projectWorkdir, version, commit string) (exeVos.VariableSet, error) {
	shortHash := commit
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}
	return exeVos.NewVariableSetFromMap(map[string]string{
		"project_version":       version,
		"project_revision":      shortHash,
		"project_revision_full": commit,
		"environment":           environment,
		"project_workdir":       projectWorkdir,
		"tool_name":             "vex",
	})
}

func (o *ExecutionOrchestrator) generateStepFingerprints(
	projectPath, environment string,
	workspace *worAgg.Workspace,
	stepName pipDom.StepName) (storageVos.FingerprintSet, error) {

	envFp, err := storageVos.NewEnvironment(environment)
	if err != nil {
		return storageVos.FingerprintSet{}, err
	}

	codeFp, err := o.fingerprintSvc.FromDirectory(projectPath)
	if err != nil {
		return storageVos.FingerprintSet{}, fmt.Errorf("generar fingerprint del proyecto: %w", err)
	}

	instructionPath := workspace.StepTemplatePath(stepName.FullName())
	instFp, err := o.fingerprintSvc.FromDirectory(instructionPath)
	if err != nil {
		return storageVos.FingerprintSet{}, fmt.Errorf("generar fingerprint de instrucciones: %w", err)
	}

	varsPath := workspace.VarsTemplatePath(environment, stepName.Name())
	varsFp, err := o.fingerprintSvc.FromFile(varsPath)
	if err != nil {
		return storageVos.FingerprintSet{}, fmt.Errorf("generar fingerprint de variables: %w", err)
	}

	fps := make(map[storageVos.FingerprintKind]storageVos.Fingerprint)

	// Solo añadimos fingerprints válidos (no vacíos) al set
	if codeFp.String() != "" {
		fps[storageVos.KindCode] = codeFp
	}
	if instFp.String() != "" {
		fps[storageVos.KindInstruction] = instFp
	}
	if varsFp.String() != "" {
		fps[storageVos.KindVars] = varsFp
	}

	return storageVos.NewFingerprintSet(fps, envFp), nil
}

var _ = (*ExecutionOrchestrator)(nil)
