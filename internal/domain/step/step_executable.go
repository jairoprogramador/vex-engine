package step

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// StepExecutable posee el ciclo before/exec/after de un step y VE su error, así
// que es quien emite `step_started` y `step_finished` (spec 19 §5.1).
//
// # El runner ANOTA; el ejecutable EMITE
//
// La decisión de saltar un step no vive aquí sino en `StepRunnerHandler`, y la
// frontera se mantiene en esa dirección a propósito (§5.1'): el runner *decide*
// y deja la decisión anotada en el request —motivo, huella para informar,
// evidencia—, el ejecutable *ejecuta, observa y publica*. Al revés —el runner
// emitiendo— el hecho lo pondría quien no ve el error del ciclo, y un step que
// falla en su limpieza se registraría como exitoso.
//
// Funciona porque el `StepRequestHandler` se crea en el ámbito de este método,
// así que los dos closures y el cierre lo ven.
type StepExecutable struct {
	command.BaseExecutable
	handler   StepHandler
	records   state.Records
	recordIDs state.RecordIDFactory
	entries   cache.Entries
	facts     FactSink
}

var _ command.Executable = (*StepExecutable)(nil)

func NewStepExecutable(
	handler StepHandler,
	records state.Records,
	recordIDs state.RecordIDFactory,
	entries cache.Entries,
	facts FactSink) *StepExecutable {

	return &StepExecutable{
		handler:   handler,
		records:   records,
		recordIDs: recordIDs,
		entries:   entries,
		facts:     facts,
	}
}

func (s *StepExecutable) Execute(executionContext *command.ExecutionContext) error {
	// El request se compone AQUÍ y no dentro del `exec` porque el cierre del par
	// tiene que poder leer lo que el runner anotó en él, y el cierre corre incluso
	// cuando el `exec` no llegó a correr.
	request := NewStepRequestHandler(executionContext, executionContext.StepName())

	var startedAt time.Time
	opened := false

	return s.Run(
		executionContext,
		func() error {
			// El hecho de apertura es lo PRIMERO del `before`, y el instante se toma
			// antes de emitirlo: la duración mide el step, no lo que cueste escribir
			// su hecho.
			//
			// Aquí vivía `Emit("Step … en ejecución")`. La línea no desaparece: se
			// DERIVA de este hecho (§5.5), que es la inversión que la spec compra —el
			// registro pasa a ser la fuente y la narrativa su proyección—.
			startedAt = executionContext.Now()
			if err := s.facts.StepStarted(executionContext.Ctx(), StepStartedFact{
				StepID: executionContext.StepFullName(),
			}); err != nil {
				return err
			}
			opened = true

			// La etapa que el portal consume deja de ser un vocabulario paralelo sin
			// llamadores (BL-5/R-10): la emite el dueño del hecho, en el mismo punto
			// y derivada de él. Se CABLEA y no se borra porque `--status-endpoint`
			// sobrevive (spec 16) y es el canal del portal; se va cuando el sink
			// `http` lo subsuma (spec 26).
			executionContext.NotifyStage("running_step:" + executionContext.StepFullName())

			executionContext.ResetFileSessions()
			// La cuenta de lo PRODUCIDO se abre aquí, con la del step: lo que dejó
			// el step anterior ya está en su registro y en el mapa acumulado, y
			// arrastrarlo hasta éste sería volver al espacio plano (spec 14 §6).
			executionContext.ResetProducedVars()
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
			err := s.handler.Handle(request.Ctx(), request)
			switch {
			case err == nil && request.WasSkipped():
				// El step no ejecutó nada, así que no hay hecho que registrar: el
				// almacén dejaría escrito «esto corrió» sobre cero evidencia, y eso
				// es lo que hacía que un `commands.yaml` vacío no se pudiera volver
				// a intentar nunca (spec 04 §5.3).
				//
				// La línea que aquí se emitía la deriva ahora el renderizador del
				// `step_finished{SKIPPED}` que este camino produce.

			case err == nil && !request.WasExecuted():
				// El step revivió: sus comandos no corrieron porque el último
				// registro de su clave dice que ese trabajo ya está hecho. Es un
				// éxito, y NO es un hecho nuevo — escribir un registro aquí lo
				// duplicaría en cada corrida y, como el camino de revivir no anota
				// huella, el registro nuevo dejaría al step sin poder revivir nunca
				// más (spec 11 §5.3).
				//
				// Se marca `CACHED` y no `SUCCESS`: desde la spec 19 el status
				// alimenta el hecho, y «se ejecutó correctamente» sobre cero comandos
				// sería una conclusión.
				request.MarkStepCached()

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
				//
				// La cabecera del fallo la deriva el renderizador del
				// `step_finished{FAILURE}`; el TEXTO del error se sigue emitiendo por
				// el canal de líneas porque es diagnóstico y su hecho equivalente —el
				// extracto acotado y redactado— es de la spec 20.
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
		// El cierre del par, y corre SIEMPRE: también cuando el `before` falló
		// después de haber emitido su apertura, que es el camino que la garantía de
		// la spec 06 no cubre y que dejaba el par abierto (§5.2', 06 §9.5).
		//
		// Recibe el error DEFINITIVO —incluido el de la limpieza— porque es el que
		// el hecho tiene que contar: un step cuyo `terraform apply` fue bien y cuya
		// plantilla no se pudo restaurar no terminó bien.
		func(err error) error {
			if !opened {
				// Lo que nunca se abrió no se cierra. Emitir sólo el cierre sería
				// inventar un hueco en vez de taparlo.
				return nil
			}
			return s.facts.StepFinished(executionContext.Ctx(), StepFinishedFact{
				StepID:          executionContext.StepFullName(),
				Scope:           request.StateScope(),
				Status:          stepStatusOf(request, err),
				Duration:        executionContext.Now().Sub(startedAt),
				FromCache:       err == nil && !request.WasSkipped() && !request.WasExecuted(),
				Reason:          request.Reason(),
				StepFingerprint: request.ReportedFingerprint(),
				Evidence:        request.Evidence(),
				Err:             err,
			})
		},
	)
}

// stepStatusOf lee el status que el request ya calculaba y NADIE leía a medias
// (BL-4): `StepExecutable` miraba el `SKIPPED` para no persistir, y ahí se
// acababa su vida.
//
// El error manda sobre lo anotado, y no es un caso teórico: `exec` puede
// terminar bien y fallar la limpieza después. Un step cuyo ciclo acabó en error
// no terminó con éxito por mucho que su cadena lo marcara.
func stepStatusOf(request *StepRequestHandler, err error) command.StepStatus {
	if err != nil {
		return command.StepFailure
	}
	return request.StepStatus()
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
// Un step que no DECLARA no escribe nada, y son dos ausencias con la misma
// consecuencia (ver `StepConfig.Remembers`):
//
//   - sin `config.yaml` no declara ámbito, luego no hay dónde (spec 13 §5.3), y
//     eso es lo que impide que el motor le invente uno;
//   - sin `rules` no hay afirmación que guardar (spec 15 §5.5). El step se
//     ejecuta en cada corrida, así que un registro suyo no podría revivir nada;
//     lo único que haría es crecer para siempre.
//
// Ninguna de las dos es un caso de error.
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

	if !request.StepConfig().Remembers() {
		return
	}

	key, _, err := request.StateKey()
	if err != nil {
		s.warn(executionContext, err)
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
		producedVariables(request), request.StepFingerprint(), producedBy)

	// El índice apunta a un registro que YA existe: si el registro no se pudo
	// escribir, no hay a qué apuntar y no se escribe entrada. Un índice con
	// punteros rotos dejaría de ser reconstruible sin distinguir cuáles lo están.
	if recordID.IsZero() {
		return
	}
	s.putIndexEntry(request, executionContext, key, recordID)

	// LA MITAD SIMÉTRICA de la evidencia (spec 19 §5.1, 28 §5.3): el registro que
	// estuvo vigente para este step viaja en su hecho de cierre **también cuando
	// el step ejecutó** —el que acaba de escribir—, y no sólo cuando revivió. Sin
	// ella, un rollback anclado a una ejecución pasada tendría que derivar por
	// fechas qué registro estuvo vigente en cada step, que es guardar una
	// conclusión en vez de un hecho.
	//
	// Se anota DESPUÉS de escribir y sólo si se escribió: una evidencia que apunta
	// a un registro que no llegó al disco afirma que hay algo detrás y no deja
	// llegar hasta él, que es peor que no afirmar nada (`EvidenceRef.Validate`).
	request.RecordEvidence(EvidenceFact{
		ExecutionID: producedBy.ExecutionID,
		At:          producedBy.At,
		StateKey:    key,
		RecordID:    recordID,
	})
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

// producedVariables es el conjunto que va al registro: lo que ESTE step
// PRODUJO, no todo lo que vio.
//
// Es el reparto consume/produce de la spec 14 §6, y es la mitad sin la cual esta
// spec no cierra ninguno de sus dos defectos. Aquí se persistía el mapa acumulado
// ENTERO —se llamaba `persistableVariables`— y de ahí salían los dos:
//
//   - un literal declarado entraba en el almacén en la primera corrida y volvía
//     como `OriginState` en la segunda, por encima de `OriginDeclared`, así que
//     EDITARLO EN EL PIPELINECODE DEJABA DE SURTIR EFECTO —y el step ni se
//     re-ejecutaba, porque la huella tampoco cambiaba— (spec 12 §9.2);
//   - y las salidas de una corrida entraban como entradas de la siguiente, que es
//     la raíz de que todo step con `outputs` se re-ejecute una vez de más. Esa
//     mitad la termina de cerrar la spec 27, cuando el material de la huella pase
//     de «acumulado resuelto» a «declarado»: aquí se le quita al almacén la copia
//     de lo consumido, allí se le quita a la huella.
//
// Lo que el step consume no se pierde: viaja por el mapa acumulado durante la
// corrida, y entre corridas vuelve de donde vino —el registro de su productor, o
// el literal del pipelinecode—. Lo que deja de ocurrir es que cada step guarde
// una copia de todo lo que pasó por delante.
//
// El filtro de volátiles se conserva aunque hoy ninguna salida pueda serlo —haría
// falta un `probe` que capturara `project_version`—: la lista tiene un solo dueño
// y está especificada en `fingerprint/SPEC-VARIABLES-v1.md` §3.1, y aplicarla
// aquí es más barato que razonar cada vez sobre si alguien puede alcanzarla.
//
// Desapareció con el modelo la comparación previa por `reflect.DeepEqual`: en un
// almacén append-only no hay nada que comparar antes de escribir, porque no se
// está decidiendo si sustituir algo.
// El orden por nombre es para el humano que abre el archivo: un mapa de Go se
// recorre en orden aleatorio, y sin ordenar dos registros con las mismas
// variables se verían distintos en un diff. No es material de ninguna huella.
func producedVariables(request *StepRequestHandler) []command.Variable {
	variables := request.ProducedVars().Filter(
		func(variable command.Variable) bool {
			return !command.IsVolatileVar(variable.Name())
		}).ToSlice()

	slices.SortFunc(variables, func(a, b command.Variable) int {
		return strings.Compare(a.Name(), b.Name())
	})
	return variables
}
