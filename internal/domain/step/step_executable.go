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
	handler        StepHandler
	varsRepository VarsStoreRepository
	statusWriter   *domStepStatus.StatusWriter
}

var _ command.Executable = (*StepExecutable)(nil)

func NewStepExecutable(
	handler StepHandler,
	varsRepository VarsStoreRepository,
	statusWriter *domStepStatus.StatusWriter) *StepExecutable {

	return &StepExecutable{
		handler:        handler,
		varsRepository: varsRepository,
		statusWriter:   statusWriter,
	}
}

func (s *StepExecutable) Execute(executionContext *command.ExecutionContext) error {
	return s.Run(
		executionContext,
		func() error {
			executionContext.Emit("Step " + executionContext.StepName() + " en ejecución")
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
			switch {
			case err == nil && request.WasSkipped():
				// El step no ejecutó nada, así que no hay estado que persistir: el
				// almacén dejaría escrito «sin cambios» sobre cero evidencia, y eso
				// es lo que hacía que un `commands.yaml` vacío no se pudiera volver
				// a intentar nunca (spec 04 §5.3).
				executionContext.Emit(fmt.Sprintf("Step %s saltado: %s",
					executionContext.StepName(), request.SkipReason()))

			case err == nil:
				request.MarkStepSuccess()

				// AQUÍ, y solo aquí, se escribe el estado de re-ejecución: después
				// de que el último comando del step terminó bien (spec 09 §5.2).
				//
				// Antes lo escribía cada regla dentro de su `Evaluate`, antes de
				// ejecutar nada, y el borrado compensatorio del camino de error
				// intentaba revertirlo. El compensador no podía cubrir la muerte
				// dura —es código que corre después—, así que un SIGKILL o un OOM
				// a mitad dejaba escrito «sin cambios» para un step que nunca
				// terminó y la corrida siguiente lo saltaba. Con la escritura
				// aquí no hay nada que compensar, y el `Delete` desapareció junto
				// con su causa.
				if writeErr := s.statusWriter.Write(
					request.StatusContext(), request.StatusEvidences()); writeErr != nil {
					// Fail-open y VISIBLE: no haber podido guardar el caché no
					// invalida el despliegue que sí ocurrió, así que el step no
					// falla por esto. Lo que sí pasa es que la corrida siguiente
					// volverá a ejecutarlo, y el usuario merece saber que su
					// caché está roto en vez de creer que su código cambió
					// (spec 09 §2).
					executionContext.Emit(fmt.Sprintf(
						"advertencia: no se pudo guardar el estado de re-ejecución del step %s: %v",
						executionContext.StepName(), writeErr))
				}

				err := s.saveScopeVars(executionContext.Environment(), executionContext.StepName(), executionContext)
				if err != nil {
					executionContext.Emit(fmt.Sprintf("error al guardar vars scope %s: %v", executionContext.Environment(), err))
				}
				err = s.saveScopeVars(command.SharedScopeName, executionContext.StepName(), executionContext)
				if err != nil {
					executionContext.Emit(fmt.Sprintf("error al guardar vars scope %s: %v", command.SharedScopeName, err))
				}
				// step es "deploy" crear tag en repo git con la version actual

			default:
				// Un step fallido no borra nada porque no había escrito nada: la
				// evidencia que la policy le anotó muere aquí con la cadena
				// (spec 09 §5.2).
				//
				// Aquí vivía el borrado compensatorio, y con él el descarte de
				// error que la spec 02 §5.4 tuvo que parchear para que dejara de
				// mentir. La 02 hizo que dijera la verdad; esta lo elimina, que es
				// mejor que arreglarlo: un compensador solo revierte lo que el
				// programa alcanza a ejecutar, y la muerte dura —la que de verdad
				// dejaba un `deploy` saltado— nunca pasa por él.
				executionContext.Emit("Step " + executionContext.StepName() + " ejecución fallida:")
				executionContext.Emit(err.Error())
			}
			return err
		},
		// Limpieza garantizada: corre falle o no el step (spec 06 §5.1).
		//
		// La variable se retira ANTES de restaurar porque restaurar puede fallar
		// y salir por el `return`: `step_workdir` es material del mapa acumulado,
		// del que salen la huella y la identidad del step, y dejar ahí el del
		// step fallido es dejar residuo en la identidad. El orden hace que ese
		// invariante no dependa del disco.
		func() error {
			executionContext.RemoveAccumulatedVar(command.VarStepWorkdir)
			if err := executionContext.RestoreFileSessions(); err != nil {
				return fmt.Errorf("restaurar las plantillas del step %s: %w",
					executionContext.StepName(), err)
			}
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
				variable.Name() != command.VarToolName &&
				variable.Name() != command.VarProjectWorkdir &&
				variable.Name() != command.VarStepWorkdir
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
