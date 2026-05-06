package step

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

type StepExecutable struct {
	command.BaseExecutable
	handler          StepHandler
	varsRepository   VarsStoreRepository
	statusRepository domStepStatus.StatusRepository
}

var _ command.Executable = (*StepExecutable)(nil)

func NewStepExecutable(
	handler StepHandler,
	varsRepository VarsStoreRepository,
	statusRepository domStepStatus.StatusRepository) *StepExecutable {

	return &StepExecutable{
		handler:          handler,
		varsRepository:   varsRepository,
		statusRepository: statusRepository,
	}
}

func (s *StepExecutable) Execute(executionContext *command.ExecutionContext) error {
	return s.Run(
		executionContext,
		func() error {
			executionContext.NotifyStage("running_step:" + executionContext.StepName())
			executionContext.ResetFileSessions()
			stepWorkdir := filepath.Join(executionContext.Workdir(), "steps", executionContext.StepFullName())
			stepWorkdirVariable, err := command.NewVariable(command.VarStepWorkdir, stepWorkdir, false)
			if err != nil {
				return fmt.Errorf("crear variable de step workdir: %w", err)
			}
			executionContext.AddAccumulatedVar(stepWorkdirVariable)
			return nil
		},
		func() error {
			request := NewStepRequestHandler(executionContext, executionContext.StepName())
			err := s.handler.Handle(request.Ctx(), request)
			if err == nil {
				request.MarkStepSuccess()
				err := s.saveScopeVars(executionContext.Environment(), executionContext.StepName(), executionContext)
				if err != nil {
					executionContext.Emit(fmt.Sprintf("error al guardar vars scope %s: %v", executionContext.Environment(), err))
				}
				err = s.saveScopeVars(command.SharedScopeName, executionContext.StepName(), executionContext)
				if err != nil {
					executionContext.Emit(fmt.Sprintf("error al guardar vars scope %s: %v", command.SharedScopeName, err))
				}
				// step es "deploy" crear tag en repo git con la version actual
			} else {
				s.statusRepository.Delete(executionContext.ProjectUrl(), executionContext.PipelineUrl(), executionContext.Environment(), executionContext.StepName())
			}
			return err
		},
		func() error {
			executionContext.RestoreFileSessions()
			executionContext.RemoveAccumulatedVar(command.VarStepWorkdir)
			return nil
		},
	)
}

func (s *StepExecutable) saveScopeVars(
	scope, step string,
	executionContext *command.ExecutionContext) error {

	isShared := scope == command.SharedScopeName

	repositoryScopeVars, err := s.varsRepository.Get(executionContext.Ctx(), executionContext.ProjectUrl(), executionContext.PipelineUrl(), scope, step)
	if err != nil {
		executionContext.Emit(fmt.Sprintf("error al cargar vars scope %s: %v", scope, err))
		return nil
	}

	accumulatedScopeVars := executionContext.FilteredAccumulatedVars(
		func(variable command.Variable) bool {
			return variable.IsShared() == isShared &&
				variable.Name() != command.VarProjectVersion &&
				variable.Name() != command.VarProjectRevision &&
				variable.Name() != command.VarProjectRevisionFull &&
				variable.Name() != command.VarToolName
		}).ToSlice()

	accumulatedScopeVars = sortedExecutionVarsByName(accumulatedScopeVars)
	repositoryScopeVars = sortedExecutionVarsByName(repositoryScopeVars)

	if !reflect.DeepEqual(repositoryScopeVars, accumulatedScopeVars) {
		return s.varsRepository.Save(executionContext.Ctx(), executionContext.ProjectUrl(), executionContext.PipelineUrl(), scope, step, accumulatedScopeVars)
	}
	return nil
}

func sortedExecutionVarsByName(vars []command.Variable) []command.Variable {
	ordered := slices.Clone(vars)
	slices.SortFunc(ordered, func(a, b command.Variable) int {
		if a.Name() < b.Name() {
			return -1
		}
		if a.Name() > b.Name() {
			return 1
		}
		return 0
	})
	return ordered
}
