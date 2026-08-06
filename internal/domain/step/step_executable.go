package step

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type StepExecutable struct {
	command.BaseExecutable
	handler        StepHandler
	varsRepository VarsStoreRepository
	entries        cache.Entries
}

var _ command.Executable = (*StepExecutable)(nil)

func NewStepExecutable(
	handler StepHandler,
	varsRepository VarsStoreRepository,
	entries cache.Entries) *StepExecutable {

	return &StepExecutable{
		handler:        handler,
		varsRepository: varsRepository,
		entries:        entries,
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

				// AQUÍ, y solo aquí, se escribe la entrada de caché: después de
				// que el último comando del step terminó bien (spec 09 §5.2,
				// heredado por la 10).
				//
				// Antes lo escribía cada regla dentro de su `Evaluate`, antes de
				// ejecutar nada, y el borrado compensatorio del camino de error
				// intentaba revertirlo. El compensador no podía cubrir la muerte
				// dura —es código que corre después—, así que un SIGKILL o un OOM
				// a mitad dejaba escrito «sin cambios» para un step que nunca
				// terminó y la corrida siguiente lo saltaba. Con la escritura
				// aquí no hay nada que compensar.
				//
				// Lo que la spec 10 cambió es sólo QUÉ se escribe: donde la 09
				// dejó un `switch` por nombre de regla repartiendo cuatro
				// evidencias a cuatro almacenes, hay una sola escritura.
				s.putCacheEntry(request, executionContext)

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

// putCacheEntry deja constancia de que ESTE contenido exacto ya se ejecutó con
// éxito aquí.
//
// Sin clave anotada no se escribe nada, y eso cubre dos casos distintos con la
// misma respuesta: el paso se saltó por caché —no hay nada nuevo que decir— o no
// se pudo componer su material. En el segundo, escribir sería peor que no
// escribir: dejaría una entrada bajo una clave incompleta, que colisiona con la
// de cualquier otro material al que le falte lo mismo. Como «ausencia de entrada
// ⇒ ejecutar», no escribir es exactamente lo correcto.
func (s *StepExecutable) putCacheEntry(
	request *StepRequestHandler,
	executionContext *command.ExecutionContext) {

	key, ok := request.CacheKey()
	if !ok {
		return
	}

	// El instante sale del reloj inyectable a través del agregado (spec 07): no
	// hay `time.Now()` en el dominio, y por eso los dos lados del borde del TTL
	// se pueden probar sin esperar treinta días.
	entry := cache.NewEntry(cache.Provenance{
		ExecutionID: executionContext.ExecutionID().String(),
		At:          executionContext.StartedAt(),
	}, cache.DefaultTTL)

	if err := s.entries.Put(executionContext.Ctx(), key, entry); err != nil {
		// Fail-open y VISIBLE: no haber podido guardar el caché no invalida el
		// despliegue que sí ocurrió, así que el step no falla por esto —convertir
		// «no pude guardar el caché» en «el despliegue falló» sería mentir en la
		// dirección peligrosa (spec 09 §9.6)—. Lo que sí pasa es que la corrida
		// siguiente volverá a ejecutarlo, y el usuario merece saber que su caché
		// está roto en vez de creer que su código cambió.
		executionContext.Emit(fmt.Sprintf(
			"advertencia: no se pudo guardar el estado de re-ejecución del step %s: %v",
			executionContext.StepName(), err))
	}
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

	// La lista de volátiles ya no está escrita aquí a mano: era la misma que
	// filtraba la huella de variables, duplicada en dos archivos sin nada que
	// las mantuviera sincronizadas (spec 10). Ahora hay un solo dueño, y está
	// especificada en `fingerprint/SPEC-VARIABLES-v1.md` §3.1.
	accumulatedScopeVars := executionContext.FilteredAccumulatedVars(
		func(variable command.Variable) bool {
			return variable.IsShared() == isShared && !command.IsVolatileVar(variable.Name())
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
