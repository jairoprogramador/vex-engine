package application

/*
import (
	"context"
	"fmt"
	"sync"

	execAggr "github.com/jairoprogramador/vex-engine/internal/domain/execution/handlers"
	expipeline "github.com/jairoprogramador/vex-engine/internal/domain/execution/pipeline"

	pipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	pipeServ "github.com/jairoprogramador/vex-engine/internal/domain/pipeline/services"

	projects "github.com/jairoprogramador/vex-engine/internal/domain/project"
	projPort "github.com/jairoprogramador/vex-engine/internal/domain/project/ports"
	projServ "github.com/jairoprogramador/vex-engine/internal/domain/project/services"

	statPort "github.com/jairoprogramador/vex-engine/internal/domain/storage/ports"
	statServ "github.com/jairoprogramador/vex-engine/internal/domain/storage/services"
	statVobs "github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"

	iGitRepo "github.com/jairoprogramador/vex-engine/internal/infrastructure/git"
	iUtils "github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"

	applDtos "github.com/jairoprogramador/vex-engine/internal/application/dto"
	applMapp "github.com/jairoprogramador/vex-engine/internal/application/mappers"
)

type ExecutionOrchestrator struct {
	rootVexPath          string
	pipelinesBasePath    string
	projectsBasePath     string
	storageBasePath      string
	workdirBasePath      string
	gitRepository        iGitRepo.GitRepository
	versionResolver      *projServ.VersionResolver
	projectGitRepository projPort.ProjectRepository
	planResolver         *pipeServ.PlanResolver
	fingerprintSvc       statPort.FingerprintFilesystem
	decider              *statServ.ExecutionDecider
	pipelineRunner       *expipeline.PipelineRunner
	executionRepository  execAggr.ExecutionRepository
	// liveExecutions mantiene los agregados activos en memoria para poder cancelarlos.
	//liveExecutions sync.Map // map[string]*exeAgg.Execution
}

func NewExecutionOrchestrator(
	rootVexPath string,
	pipelinesBasePath string,
	projectsBasePath string,
	storageBasePath string,
	workdirBasePath string,
	gitRepository iGitRepo.GitRepository,
	versionResolver *projServ.VersionResolver,
	projectGitRepository projPort.ProjectRepository,
	planResolver *pipeServ.PlanResolver,
	fingerprintSvc statPort.FingerprintFilesystem,
	decider *statServ.ExecutionDecider,
	pipelineRunner *expipeline.PipelineRunner,
	executionRepository execAggr.ExecutionRepository,
) *ExecutionOrchestrator {
	return &ExecutionOrchestrator{
		rootVexPath:          rootVexPath,
		pipelinesBasePath:    pipelinesBasePath,
		projectsBasePath:     projectsBasePath,
		storageBasePath:      storageBasePath,
		workdirBasePath:      workdirBasePath,
		gitRepository:        gitRepository,
		versionResolver:      versionResolver,
		projectGitRepository: projectGitRepository,
		planResolver:         planResolver,
		fingerprintSvc:       fingerprintSvc,
		decider:              decider,
		pipelineRunner:       pipelineRunner,
		executionRepository:  executionRepository,
	}
}

func (o *ExecutionOrchestrator) CreateExecution(ctx context.Context, request applDtos.RequestInput) (execAggr.ExecutionID, error) {
	execution, err := applMapp.MapToExecution(request)
	if err != nil {
		return execAggr.ExecutionID{}, fmt.Errorf("execution orchestrator: mapear ejecución: %w", err)
	}

	if err := o.executionRepository.Save(ctx, execution); err != nil {
		return execAggr.ExecutionID{}, fmt.Errorf("execution orchestrator: guardar ejecución inicial: %w", err)
	}

	childCtx, cancelFn := context.WithCancel(context.Background())
	execution.SetCancelFn(cancelFn)

	//o.liveExecutions.Store(execution.ID().String(), execution)

	go o.runExecution(childCtx, execution.ID(), request)

	return execution.ID(), nil
}

func (o *ExecutionOrchestrator) Cancel(ctx context.Context, executionID execAggr.ExecutionID) error {
	//val, ok := o.liveExecutions.Load(executionID.String())
	if !ok {
		return fmt.Errorf("execution orchestrator: cancel: execution %s not found in memory", executionID.String())
	}

	execution, ok := val.(*execAggr.Execution)
	if !ok {
		return fmt.Errorf("execution orchestrator: cancel: unexpected type in live executions map")
	}

	execution.Cancel()
	o.liveExecutions.Delete(executionID.String())

	if err := o.executionRepository.UpdateStatus(ctx, executionID, execAggr.StatusCancelled); err != nil {
		return fmt.Errorf("execution orchestrator: cancel: update status: %w", err)
	}

	return nil
}

// runExecution corre en goroutine y ejecuta el pipeline completo.
func (o *ExecutionOrchestrator) runExecution(
	ctx context.Context, executionID execAggr.ExecutionID, request applDtos.RequestInput) {

	//defer o.liveExecutions.Delete(executionID.String())

	emit := func(line string) {
		o.pipelineRunner.Emitter().Emit(executionID, line)
	}

	if err := o.executionRepository.UpdateStatus(ctx, executionID, execAggr.StatusRunning); err != nil {
		emit(fmt.Sprintf("ERROR: no se pudo actualizar el estado a running: %v", err))
	}

	if err := o.runPipeline(ctx, executionID, request); err != nil {
		emit(fmt.Sprintf("ERROR: %v", err))
		_ = o.executionRepository.UpdateStatus(ctx, executionID, execAggr.StatusFailed)
		return
	}

	if err := o.executionRepository.UpdateStatus(ctx, executionID, execAggr.StatusSucceeded); err != nil {
		emit(fmt.Sprintf("ADVERTENCIA: ejecución exitosa pero no se pudo actualizar el estado: %v", err))
	}
	emit("Ejecución completada con éxito.")
}

func (o *ExecutionOrchestrator) runPipeline(ctx context.Context, executionID execAggr.ExecutionID, request applDtos.RequestInput) error {
	emit := func(line string) {
		o.pipelineRunner.Emitter().Emit(executionID, line)
	}

	project, err := applMapp.MapToProject(request)
	if err != nil {
		return fmt.Errorf("construir proyecto: %w", err)
	}

	pipelineUrl, err := pipeline.NewPipelineURL(request.Pipeline.Url)
	if err != nil {
		return fmt.Errorf("NewPipelineURL: url inválida: %w", err)
	}

	refPipeline, err := pipeline.NewPipelineRef(request.Pipeline.Ref)
	if err != nil {
		return fmt.Errorf("NewPipelineURL: ref inválida: %w", err)
	}

	projectLocalPath, err := o.gitRepository.Clone(ctx, project.Url(), project.Ref().String(), o.projectsBasePath)
	if err != nil {
		return fmt.Errorf("clonar repositorio del proyecto: %w", err)
	}

	pipelineLocalPath, err := o.gitRepository.Clone(ctx, pipelineUrl, refPipeline.String(), o.pipelinesBasePath)
	if err != nil {
		return fmt.Errorf("clonar repositorio pipeline: %w", err)
	}

	pipelinePlan, err := o.loadPlanPipeline(ctx, pipelineLocalPath, request.Execution.Environment, request.Execution.Step)
	if err != nil {
		return fmt.Errorf("construir plan de pipeline: %w", err)
	}

	environmentValue := pipelinePlan.Environment().Value()

	projectVars, err := o.prepareProjectVariables(project)
	if err != nil {
		return fmt.Errorf("preparar variables del proyecto: %w", err)
	}

	othersVars, err := o.prepareOthersVariables(ctx, environmentValue, projectLocalPath)
	if err != nil {
		return fmt.Errorf("preparar variables de entorno: %w", err)
	}

	cumulativeVars := make(execAggr.VariablesMap)
	cumulativeVars.AddAllMap(projectVars)
	cumulativeVars.AddAllMap(othersVars)

	codeFingerprint, err := o.fingerprintSvc.FromDirectory(projectLocalPath)
	if err != nil {
		return fmt.Errorf("generar fingerprint del proyecto: %w", err)
	}

	if codeFingerprint.String() == "" {
		return fmt.Errorf("no se pudo generar el fingerprint del proyecto")
	}

	version, ok := cumulativeVars.Get(execAggr.VarProjectVersion)
	if !ok {
		return fmt.Errorf("variable de versión no encontrada")
	}
	commitHash, ok := cumulativeVars.Get(execAggr.VarProjectRevisionFull)
	if !ok {
		return fmt.Errorf("variable de commit no encontrada")
	}

	emit("Iniciando la ejecución del plan...")
	emit(fmt.Sprintf("  - Entorno: %s", environmentValue))
	emit(fmt.Sprintf("  - Versión: %s", version.Value()))
	emit(fmt.Sprintf("  - Commit: %s", commitHash.Value()))

	for _, pipelineStep := range pipelinePlan.Steps() {
		stepName := pipelineStep.StepName().Name()
		emit(fmt.Sprintf("Ejecutando paso %s ...", pipelineStep.StepName().Name()))

		fingerprints, err := o.fingerprintsGenerator(pipelineLocalPath, environmentValue, pipelineStep.StepName())
		if err != nil {
			return fmt.Errorf("generar fingerprint para el paso '%s': %w", stepName, err)
		}
		fingerprints.Add(statVobs.KindCode, codeFingerprint)

		statusKey := statVobs.NewStorageKey(environmentValue, stepName, project.Url(), pipelineUrl)

		decision, err := o.decider.Decide(ctx, statusKey, fingerprints)
		if err != nil {
			return fmt.Errorf("decidir ejecución del paso '%s': %w", stepName, err)
		}

		executionStep, err := applMapp.MapToStepExecution(pipelineStep, environmentValue, project.Url(),
			pipelineUrl, o.pipelinesBasePath, o.workdirBasePath, o.storageBasePath)
		if err != nil {
			return fmt.Errorf("construir entrada del paso '%s': %w", stepName, err)
		}

		if !decision.ShouldRun() {
			emit(fmt.Sprintf("  - Paso '%s' sin cambios en el historial de ejecución (match de %s). Omitiendo.",
				stepName, decision.MatchedAt().Format("2006-01-02 15:04:05")))

			// Aunque el paso se omite, sus vars persisten y alimentan steps siguientes
			storageStepVars, err := o.pipelineRunner.LoadStepVars(executionStep)
			if err != nil {
				return fmt.Errorf("cargar vars del paso omitido '%s': %w", stepName, err)
			}
			cumulativeVars.AddAllMap(storageStepVars)
			continue
		}

		result, err := o.pipelineRunner.RunStep(ctx, executionStep, cumulativeVars, executionID)
		if err != nil {
			emit("--- Logs del fallo ---")
			if result != nil {
				emit(result.Logs)
			}
			emit("---------------------")
			return fmt.Errorf("el paso '%s' finalizó con error: %w", stepName, err)
		}

		emit(fmt.Sprintf("  - Paso '%s' completado:\n%s", stepName, result.Logs))
		cumulativeVars.AddAllMap(result.OutputVars)

		if err := o.decider.RecordSuccess(ctx, statusKey, fingerprints); err != nil {
			emit(fmt.Sprintf("ADVERTENCIA: no se pudo guardar el historial de ejecución del paso '%s'. Se re-ejecutará la próxima vez. Error: %v",
				stepName, err))
		}
	}

	if request.Execution.Step == "deploy" {
		if err := o.projectGitRepository.CreateTagForCommit(ctx, projectLocalPath, commitHash.Value(), version.Value()); err != nil {
			emit(fmt.Sprintf("ADVERTENCIA: no se pudo crear el tag del commit. Error: %v", err))
		}
	}

	return nil
}

func (o *ExecutionOrchestrator) loadPlanPipeline(
	ctx context.Context, pipelineLocalPath, environmentValue, stepName string) (*pipeline.PipelinePlan, error) {
	limit := pipeline.NewStepLimit(stepName)
	return o.planResolver.Load(ctx, pipelineLocalPath, environmentValue, limit)
}

func (o *ExecutionOrchestrator) prepareProjectVariables(project *projects.Project) (execAggr.VariablesMap, error) {
	idStr := project.ID().String()
	if len(idStr) > 8 {
		idStr = idStr[:8]
	}
	return execAggr.NewVariablesMapFromMap(map[string]string{
		execAggr.VarProjectID:   idStr,
		execAggr.VarProjectName: project.Name().String(),
		execAggr.VarProjectOrg:  project.Org().String(),
		execAggr.VarProjectTeam: project.Team().String(),
	})
}

func (o *ExecutionOrchestrator) prepareOthersVariables(ctx context.Context, environmentValue, projectLocalPath string) (execAggr.VariablesMap, error) {
	version, commitHash, err := o.versionResolver.NextVersion(ctx, projectLocalPath)
	if err != nil {
		return nil, fmt.Errorf("calcular versión: %w", err)
	}

	shortHash := commitHash
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}
	return execAggr.NewVariablesMapFromMap(map[string]string{
		execAggr.VarProjectVersion:      version.String(),
		execAggr.VarProjectRevision:     shortHash,
		execAggr.VarProjectRevisionFull: commitHash,
		execAggr.VarEnvironment:         environmentValue,
		execAggr.VarProjectWorkdir:      projectLocalPath,
		execAggr.VarToolName:            "vex",
	})
}

func (o *ExecutionOrchestrator) fingerprintsGenerator(
	pipelineLocalPath, environmentValue string,
	stepName pipeline.StepName) (statVobs.FingerprintSet, error) {

	environmentStatus, err := statVobs.NewEnvironment(environmentValue)
	if err != nil {
		return statVobs.FingerprintSet{}, err
	}

	instFingerprint, err := o.fingerprintSvc.FromDirectory(iUtils.StepPipelinePath(pipelineLocalPath, stepName.FullName()))
	if err != nil {
		return statVobs.FingerprintSet{}, fmt.Errorf("generar fingerprint de instrucciones: %w", err)
	}

	varsFingerprint, err := o.fingerprintSvc.FromFile(iUtils.VarsPipelineFilePath(pipelineLocalPath, environmentValue, stepName.Name()))
	if err != nil {
		return statVobs.FingerprintSet{}, fmt.Errorf("generar fingerprint de variables: %w", err)
	}

	fps := make(map[statVobs.FingerprintKind]statVobs.Fingerprint)
	if instFingerprint.String() != "" {
		fps[statVobs.KindInstruction] = instFingerprint
	}
	if varsFingerprint.String() != "" {
		fps[statVobs.KindVars] = varsFingerprint
	}

	return statVobs.NewFingerprintSet(fps, environmentStatus), nil
}

var _ = (*ExecutionOrchestrator)(nil)
*/
