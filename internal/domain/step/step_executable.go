package step

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

type StepExecutable struct {
	command.BaseExecutable
	handler   StepHandler
	records   state.Records
	recordIDs state.RecordIDFactory
	entries   cache.Entries
}

var _ command.Executable = (*StepExecutable)(nil)

func NewStepExecutable(
	handler StepHandler,
	records state.Records,
	recordIDs state.RecordIDFactory,
	entries cache.Entries) *StepExecutable {

	return &StepExecutable{
		handler:   handler,
		records:   records,
		recordIDs: recordIDs,
		entries:   entries,
	}
}

func (s *StepExecutable) Execute(executionContext *command.ExecutionContext) error {
	return s.Run(
		executionContext,
		func() error {
			executionContext.Emit("Step " + executionContext.StepName() + " en ejecución")
			executionContext.ResetFileSessions()
			stepWorkdir := filepath.Join(executionContext.Workdir(), "steps", executionContext.StepFullName())
			stepWorkdirVariable, err := command.NewVariable(
				command.VarStepWorkdir, stepWorkdir, command.OriginInjected)
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
				// El step no ejecutó nada, así que no hay hecho que registrar: el
				// almacén dejaría escrito «esto corrió» sobre cero evidencia, y eso
				// es lo que hacía que un `commands.yaml` vacío no se pudiera volver
				// a intentar nunca (spec 04 §5.3).
				executionContext.Emit(fmt.Sprintf("Step %s saltado: %s",
					executionContext.StepName(), request.SkipReason()))

			case err == nil && !request.WasExecuted():
				// El step revivió: sus comandos no corrieron porque el último
				// registro de su clave dice que ese trabajo ya está hecho. Es un
				// éxito, y NO es un hecho nuevo — escribir un registro aquí lo
				// duplicaría en cada corrida y, como el camino de revivir no anota
				// huella, el registro nuevo dejaría al step sin poder revivir nunca
				// más (spec 11 §5.3).
				request.MarkStepSuccess()

			case err == nil:
				request.MarkStepSuccess()

				// AQUÍ, y solo aquí, se escribe el estado: después de que el último
				// comando del step terminó bien (spec 09 §5.2, heredado por la 10 y
				// por la 11).
				//
				// Antes lo escribía cada regla dentro de su `Evaluate`, antes de
				// ejecutar nada, y el borrado compensatorio del camino de error
				// intentaba revertirlo. El compensador no podía cubrir la muerte
				// dura —es código que corre después—, así que un SIGKILL o un OOM
				// a mitad dejaba escrito «ya se hizo» para un step que nunca
				// terminó y la corrida siguiente lo saltaba. Con la escritura aquí
				// no hay nada que compensar.
				//
				// Lo que la spec 11 cambia es QUÉ se escribe y con qué regla de
				// vida: un REGISTRO nuevo por ejecución real, que no sustituye a
				// ninguno, más una entrada de índice que apunta a él. Lo que cambia
				// la spec 13 es DÓNDE: uno solo, bajo el ámbito que el step declara.
				s.appendRecord(request, executionContext)

			default:
				// Un step fallido no borra nada porque no había escrito nada: la
				// huella que la cadena anotó muere aquí con ella (spec 09 §5.2).
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

// appendRecord deja constancia de que ESTE step acaba de ejecutarse aquí, y de
// lo que produjo.
//
// UN registro, bajo el ámbito que el step DECLARA (spec 13 §5.4). Aquí se
// escribían dos —uno de proyecto con las variables marcadas `shared` y otro del
// ambiente con el resto— porque un mismo step podía producir de los dos ámbitos
// y no había forma de saber cuál era el suyo. Con el ámbito declarado la
// bifurcación desaparece con un `if`, no con una migración: el step tiene una
// identidad, luego un sitio donde recordarse.
//
// Un step SIN `config.yaml` no escribe nada. No es un caso de error: es la
// consecuencia de §5.3 —no declara ámbito, luego no hay dónde— y lo que impide
// que el motor le invente uno.
//
// Se escribe aunque el conjunto de variables esté vacío: el hecho que el
// registro guarda es «este step corrió», no «este step produjo algo», y un
// registro vacío es además la única forma de expresar «este ámbito ya no tiene
// variables» — lo que en el almacén viejo obligaba a guardar una lista vacía.
//
// Ningún fallo de escritura tumba el step: el despliegue ocurrió, y convertir
// «no pude guardar» en «el despliegue falló» sería mentir en la dirección
// peligrosa (spec 09 §9.6). Lo que sí ocurre es que se dice en voz alta.
func (s *StepExecutable) appendRecord(
	request *StepRequestHandler,
	executionContext *command.ExecutionContext) {

	key, declarado, err := request.StateKey()
	if err != nil {
		s.warn(executionContext, err)
		return
	}
	if !declarado {
		return
	}

	// El instante sale del reloj inyectable a través del agregado (spec 07): no
	// hay `time.Now()` en el dominio, y por eso los dos lados del borde de la
	// expiración se pueden probar sin esperar treinta días.
	producedBy := state.Provenance{
		ExecutionID: executionContext.ExecutionID().String(),
		At:          executionContext.StartedAt(),
	}

	recordID := s.writeRecord(executionContext, key,
		persistableVariables(executionContext), request.StepFingerprint(), producedBy)

	// El índice apunta a un registro que YA existe: si el registro no se pudo
	// escribir, no hay a qué apuntar y no se escribe entrada. Un índice con
	// punteros rotos dejaría de ser reconstruible sin distinguir cuáles lo están.
	if !recordID.IsZero() {
		s.putIndexEntry(request, executionContext, key, recordID)
	}
}

// writeRecord escribe un registro y devuelve su identificador, o el
// identificador cero si no se pudo escribir.
func (s *StepExecutable) writeRecord(
	executionContext *command.ExecutionContext,
	key state.Key,
	variables []command.Variable,
	fingerprint string,
	producedBy state.Provenance) state.RecordID {

	recordID, err := s.recordIDs.New(producedBy.At)
	if err != nil {
		s.warn(executionContext, err)
		return state.RecordID{}
	}

	record, err := state.NewStepRecord(recordID, fingerprint, variables, producedBy)
	if err != nil {
		s.warn(executionContext, err)
		return state.RecordID{}
	}

	if err := s.records.Append(executionContext.Ctx(), key, record); err != nil {
		// Fail-open y VISIBLE. A diferencia del índice, lo que se pierde aquí es
		// un HECHO: si el step extrajo el nombre de un recurso que acaba de crear
		// en la nube, el motor deja de tener su pista. El step no falla —el
		// recurso existe igual— pero esto no puede pasar en silencio.
		s.warn(executionContext, fmt.Errorf(
			"no se pudo registrar el estado del step %s en el ámbito %s: %w",
			executionContext.StepName(), key.Scope(), err))
		return state.RecordID{}
	}
	return recordID
}

// putIndexEntry deja el puntero «este contenido exacto corrió, y éste fue el
// registro».
//
// Sin huella anotada no se escribe nada, y eso cubre dos casos con la misma
// respuesta: el step revivió —no hay nada nuevo que indexar— o no se pudo
// componer su material. En el segundo, escribir sería peor que no escribir:
// dejaría una entrada bajo una clave incompleta, que colisiona con la de
// cualquier otro material al que le falte lo mismo.
//
// Que falle no cambia ninguna decisión del motor —el índice no participa en
// ninguna—, así que la advertencia dice exactamente eso y no «tu caché está
// roto».
func (s *StepExecutable) putIndexEntry(
	request *StepRequestHandler,
	executionContext *command.ExecutionContext,
	stateKey state.Key,
	recordID state.RecordID) {

	key, ok := request.IndexKey()
	if !ok {
		return
	}

	entry, err := cache.NewEntry(stateKey, recordID)
	if err != nil {
		s.warn(executionContext, err)
		return
	}

	if err := s.entries.Put(executionContext.Ctx(), key, entry); err != nil {
		executionContext.Emit(fmt.Sprintf(
			"advertencia: no se pudo indexar el registro del step %s: %v",
			executionContext.StepName(), err))
	}
}

func (s *StepExecutable) warn(executionContext *command.ExecutionContext, err error) {
	executionContext.Emit(fmt.Sprintf("advertencia: %v", err))
}

// persistableVariables es el conjunto que va al registro: el mapa acumulado no
// volátil, entero.
//
// Perdió el parámetro `shared` con la spec 13: ya no hay dos registros que
// repartir, así que no hay nada que repartir. El único filtro que queda es el de
// volátiles, y NO está escrito aquí a mano: era la misma lista que filtra la
// huella de variables, duplicada en dos archivos sin nada que las mantuviera
// sincronizadas (spec 10). Hay un solo dueño, y está especificado en
// `fingerprint/SPEC-VARIABLES-v1.md` §3.1.
//
// Desapareció con el modelo la comparación previa por `reflect.DeepEqual`: en un
// almacén append-only no hay nada que comparar antes de escribir, porque no se
// está decidiendo si sustituir algo.
// El orden por nombre es para el humano que abre el archivo: un mapa de Go se
// recorre en orden aleatorio, y sin ordenar dos registros con las mismas
// variables se verían distintos en un diff. No es material de ninguna huella.
func persistableVariables(executionContext *command.ExecutionContext) []command.Variable {
	variables := executionContext.FilteredAccumulatedVars(
		func(variable command.Variable) bool {
			return !command.IsVolatileVar(variable.Name())
		}).ToSlice()

	slices.SortFunc(variables, func(a, b command.Variable) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return variables
}
