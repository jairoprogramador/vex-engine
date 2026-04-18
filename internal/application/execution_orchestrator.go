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
	proAgg "github.com/jairoprogramador/vex-engine/internal/domain/project/aggregates"
	staPrt "github.com/jairoprogramador/vex-engine/internal/domain/state/ports"
	staVos "github.com/jairoprogramador/vex-engine/internal/domain/state/vos"
	verPrt "github.com/jairoprogramador/vex-engine/internal/domain/versioning/ports"
	worAgg "github.com/jairoprogramador/vex-engine/internal/domain/workspace/aggregates"
	gitInf "github.com/jairoprogramador/vex-engine/internal/infrastructure/git"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
)

// ExecutionOrchestrator coordina todo el ciclo de vida de una ejecución de pipeline:
// crea el agregado Execution, lo persiste, lanza la goroutine de ejecución y retorna
// el ID inmediatamente al caller (modelo no bloqueante para el daemon HTTP).
type ExecutionOrchestrator struct {
	rootVexPath       string
	projectSvc        *ProjectService
	workspaceSvc      *WorkspaceService
	gitCloner         gitInf.RepositoryGit
	versionCalculator verPrt.VersionCalculator
	fetcher           pipPrt.RepositoryFetcher
	pipelineLoader    *pipSer.PlanBuilder
	fingerprintSvc    staPrt.FingerprintService
	stateManager      staPrt.StateManager
	stepExecutor      exePrt.StepExecutor
	copyWorkdir       exePrt.CopyWorkdir
	varsRepository    exePrt.VarsRepository
	gitRepository     verPrt.GitRepository
	emitter           exePrt.LogEmitter
	executionRepo     exePrt.ExecutionRepository
	// liveExecutions mantiene los agregados activos en memoria para poder cancelarlos.
	// La persistencia (cancelFn) no viaja al storage — solo vive aquí.
	liveExecutions sync.Map // map[string]*exeAgg.Execution
}

func NewExecutionOrchestrator(
	rootVexPath string,
	projectSvc *ProjectService,
	workspaceSvc *WorkspaceService,
	gitCloner gitInf.RepositoryGit,
	versionCalculator verPrt.VersionCalculator,
	fetcher pipPrt.RepositoryFetcher,
	pipelineLoader *pipSer.PlanBuilder,
	fingerprintSvc staPrt.FingerprintService,
	stateManager staPrt.StateManager,
	stepExecutor exePrt.StepExecutor,
	copyWorkdir exePrt.CopyWorkdir,
	varsRepository exePrt.VarsRepository,
	gitRepository verPrt.GitRepository,
	emitter exePrt.LogEmitter,
	executionRepo exePrt.ExecutionRepository,
) *ExecutionOrchestrator {
	return &ExecutionOrchestrator{
		rootVexPath:       rootVexPath,
		projectSvc:        projectSvc,
		workspaceSvc:      workspaceSvc,
		gitCloner:         gitCloner,
		versionCalculator: versionCalculator,
		fetcher:           fetcher,
		pipelineLoader:    pipelineLoader,
		fingerprintSvc:    fingerprintSvc,
		stateManager:      stateManager,
		stepExecutor:      stepExecutor,
		copyWorkdir:       copyWorkdir,
		varsRepository:    varsRepository,
		gitRepository:     gitRepository,
		emitter:           emitter,
		executionRepo:     executionRepo,
	}
}

// Run crea el agregado Execution, lo persiste y lanza la goroutine de ejecución.
// Retorna el ExecutionID inmediatamente — el caller puede usarlo para hacer polling
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
// el estado en el repositorio a StatusCancelled. Si la ejecución ya no está en
// memoria (terminó o nunca existió en este proceso) retorna error.
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
// Actualiza el estado de la ejecución en el repositorio al terminar.
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
		exitCode := 1
		_ = o.executionRepo.UpdateStatus(ctx, executionID, exeVos.StatusFailed)
		// Ignoramos el error de UpdateStatus aquí porque ya estamos en el camino de error —
		// el log emitido preserva el contexto para el operador.
		_ = exitCode
		return
	}

	exitCode := 0
	_ = exitCode
	if err := o.executionRepo.UpdateStatus(ctx, executionID, exeVos.StatusSucceeded); err != nil {
		emit(fmt.Sprintf("ADVERTENCIA: ejecución exitosa pero no se pudo actualizar el estado: %v", err))
	}
	emit("Ejecución completada con éxito.")
}

// runPipeline contiene la lógica central de ejecución: carga proyecto, workspace,
// clona template, construye plan y ejecuta cada paso.
func (o *ExecutionOrchestrator) runPipeline(ctx context.Context, executionID exeVos.ExecutionID, cmd dto.RequestInput) error {
	emit := func(line string) {
		o.emitter.Emit(executionID, line)
	}

	project, projectPath, err := o.resolveProject(ctx, cmd)
	if err != nil {
		return fmt.Errorf("resolver proyecto: %w", err)
	}

	workspace, err := o.loadWorkspace(project)
	if err != nil {
		return err
	}

	templateLocalPath := workspace.TemplatePath()
	if err := o.cloneTemplate(ctx, project, templateLocalPath); err != nil {
		return err
	}

	planDef, err := o.fetchAndBuildPlan(ctx, cmd)
	if err != nil {
		return err
	}

	version, commit, err := o.versionCalculator.CalculateNextVersion(ctx, projectPath, false)
	if err != nil {
		return fmt.Errorf("calcular versión: %w", err)
	}

	environment := planDef.Environment().Value()

	projectVars, err := o.prepareProjectVariables(project)
	if err != nil {
		return fmt.Errorf("preparar variables del proyecto: %w", err)
	}

	othersVars, err := o.prepareOthersVariables(environment, projectPath, version.String(), commit.String())
	if err != nil {
		return fmt.Errorf("preparar variables de entorno: %w", err)
	}

	cumulativeVars := make(exeVos.VariableSet)
	cumulativeVars.AddAll(projectVars)
	cumulativeVars.AddAll(othersVars)

	emit("Iniciando la ejecución del plan...")
	emit(fmt.Sprintf("  - Entorno: %s", environment))
	emit(fmt.Sprintf("  - Versión: %s", version.String()))
	emit(fmt.Sprintf("  - Commit: %s", commit.String()))

	for _, stepDef := range planDef.Steps() {
		emit(fmt.Sprintf("Ejecutando paso %s ...", stepDef.Name().Name()))

		fingerprints, err := o.generateStepFingerprints(projectPath, environment, workspace, stepDef.Name())
		if err != nil {
			return fmt.Errorf("generar fingerprint para el paso '%s': %w", stepDef.Name().Name(), err)
		}

		stateTablePath, err := workspace.StateTablePath(stepDef.Name().Name())
		if err != nil {
			return fmt.Errorf("obtener ruta de estado del paso '%s': %w", stepDef.Name().Name(), err)
		}

		hasChanged, err := o.stateManager.HasStateChanged(stateTablePath, fingerprints, staVos.NewCachePolicy(0))
		if err != nil {
			return fmt.Errorf("comprobar estado del paso '%s': %w", stepDef.Name().Name(), err)
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

		if !hasChanged {
			emit(fmt.Sprintf("  - Paso '%s' sin cambios. Omitiendo.", stepDef.Name().Name()))
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

		if err := o.stateManager.UpdateState(stateTablePath, fingerprints); err != nil {
			// El paso fue exitoso. No fallamos la ejecución por esto, pero lo notificamos
			// porque la próxima ejecución repetirá el paso innecesariamente.
			emit(fmt.Sprintf("ADVERTENCIA: no se pudo guardar el estado del paso '%s'. Se re-ejecutará la próxima vez. Error: %v", stepDef.Name().Name(), err))
		}
	}

	if cmd.Execution.Step == "deploy" {
		if err := o.gitRepository.CreateTagForCommit(ctx, projectPath, commit.String(), version.String()); err != nil {
			emit(fmt.Sprintf("ADVERTENCIA: no se pudo crear el tag del commit. Error: %v", err))
		}
	}

	return nil
}

// resolveProject determina el path local del proyecto y carga el agregado Project.
//
// Estrategia de resolución:
//   - Si cmd.Project.URL != "": los datos del proyecto vienen del DTO (modelo daemon HTTP).
//     El path local se deriva del nombre del proyecto dentro del rootVexPath.
//   - Si cmd.Project.URL == "": cmd.Project.ID se trata como una ruta local absoluta
//     (comportamiento legacy compatible con el CLI síncrono anterior).
func (o *ExecutionOrchestrator) resolveProject(ctx context.Context, cmd dto.RequestInput) (*proAgg.Project, string, error) {
	if cmd.Project.URL != "" {
		projectPath := fmt.Sprintf("%s/projects/%s", o.rootVexPath, cmd.Project.Name)
		project, err := o.projectSvc.FromDTO(cmd, projectPath)
		if err != nil {
			return nil, "", fmt.Errorf("construir proyecto desde DTO: %w", err)
		}
		return project, projectPath, nil
	}

	projectPath := cmd.Project.ID
	project, err := o.projectSvc.Load(ctx, projectPath)
	if err != nil {
		return nil, "", fmt.Errorf("cargar proyecto desde filesystem: %w", err)
	}
	return project, projectPath, nil
}

func (o *ExecutionOrchestrator) loadWorkspace(project *proAgg.Project) (*worAgg.Workspace, error) {
	workspace, err := o.workspaceSvc.NewWorkspace(
		o.rootVexPath, project.Data().Name(), project.TemplateRepo().DirName())
	if err != nil {
		return nil, fmt.Errorf("cargar workspace: %w", err)
	}
	return workspace, nil
}

func (o *ExecutionOrchestrator) cloneTemplate(
	ctx context.Context, project *proAgg.Project, templateLocalPath string) error {
	if err := o.gitCloner.Clone(ctx, project.TemplateRepo().URL(),
		project.TemplateRepo().Ref(), templateLocalPath); err != nil {
		return fmt.Errorf("clonar repositorio de plantillas: %w", err)
	}
	return nil
}

func (o *ExecutionOrchestrator) fetchAndBuildPlan(ctx context.Context, cmd dto.RequestInput) (*pipDom.PipelinePlan, error) {
	repoURL, err := pipDom.NewRepositoryURL(cmd.Pipeline.URL)
	if err != nil {
		return nil, fmt.Errorf("fetchAndBuildPlan: url inválida: %w", err)
	}

	ref, err := pipDom.NewRepositoryRef(cmd.Pipeline.Ref)
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

func (o *ExecutionOrchestrator) prepareProjectVariables(project *proAgg.Project) (exeVos.VariableSet, error) {
	return exeVos.NewVariableSetFromMap(map[string]string{
		"project_id":           project.ID().String()[:8],
		"project_name":         project.Data().Name(),
		"project_organization": project.Data().Organization(),
		"project_team":         project.Data().Team(),
	})
}

func (o *ExecutionOrchestrator) prepareOthersVariables(environment, projectWorkdir, version, commit string) (exeVos.VariableSet, error) {
	return exeVos.NewVariableSetFromMap(map[string]string{
		"project_version":       version,
		"project_revision":      commit[:8],
		"project_revision_full": commit,
		"environment":           environment,
		"project_workdir":       projectWorkdir,
		"tool_name":             "vex",
	})
}

func (o *ExecutionOrchestrator) generateCodeFingerprint(projectPath string) (staVos.Fingerprint, error) {
	codeFp, err := o.fingerprintSvc.FromDirectory(projectPath)
	if err != nil {
		return staVos.Fingerprint{}, fmt.Errorf("generar fingerprint del proyecto: %w", err)
	}
	return codeFp, nil
}

func (o *ExecutionOrchestrator) generateInstructionFingerprint(templateInstPath string) (staVos.Fingerprint, error) {
	instFp, err := o.fingerprintSvc.FromDirectory(templateInstPath)
	if err != nil {
		return staVos.Fingerprint{}, fmt.Errorf("generar fingerprint de instrucciones: %w", err)
	}
	return instFp, nil
}

func (o *ExecutionOrchestrator) generateVarsFingerprint(templateVarsPath string) (staVos.Fingerprint, error) {
	varsFp, err := o.fingerprintSvc.FromFile(templateVarsPath)
	if err != nil {
		return staVos.Fingerprint{}, fmt.Errorf("generar fingerprint de variables: %w", err)
	}
	return varsFp, nil
}

func (o *ExecutionOrchestrator) generateStepFingerprints(
	projectPath, environment string,
	workspace *worAgg.Workspace,
	stepName pipDom.StepName) (staVos.CurrentStateFingerprints, error) {

	envFp, err := staVos.NewEnvironment(environment)
	if err != nil {
		return staVos.CurrentStateFingerprints{}, err
	}

	codeFp, err := o.generateCodeFingerprint(projectPath)
	if err != nil {
		return staVos.CurrentStateFingerprints{}, err
	}

	instructionPath := workspace.StepTemplatePath(stepName.FullName())
	instFp, err := o.generateInstructionFingerprint(instructionPath)
	if err != nil {
		return staVos.CurrentStateFingerprints{}, err
	}

	varsPath := workspace.VarsTemplatePath(environment, stepName.Name())
	varsFp, err := o.generateVarsFingerprint(varsPath)
	if err != nil {
		return staVos.CurrentStateFingerprints{}, err
	}

	return staVos.NewCurrentStateFingerprints(codeFp, instFp, varsFp, envFp), nil
}

var _ = (*ExecutionOrchestrator)(nil)
