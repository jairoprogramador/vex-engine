package step

import (
	"context"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"

	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

type StepRequestHandler struct {
	executionContext *command.ExecutionContext
	stepName         string
	stepStatus       command.StepStatus
	skipReason       SkipReason

	// La huella del contenido del step, esperando a que el step termine bien
	// para entrar en su registro. Vive aquí —y no en el ExecutionContext— porque
	// es estado de ESTA cadena y muere con ella; el ExecutionContext ya es
	// demasiado grande.
	//
	// Que muera con la cadena es la propiedad que se quiere, no un efecto
	// secundario: si el step no llega al final, la huella se pierde sin que nadie
	// tenga que borrarla.
	//
	// Es un `cache.CacheKey` porque hoy la huella del step ES la clave `ck-v1`
	// —las tres huellas más las cuatro dimensiones de dirección—. La spec 27 la
	// sustituye por `sf-v1`, con las dimensiones de dirección fuera; el tipo
	// cambia entonces, el papel no. Y la sustitución es segura por construcción:
	// un registro escrito con `ck-v1:` nunca revivirá contra una huella `sf-v1:`,
	// porque las cadenas difieren en el prefijo.
	stepFingerprint cache.CacheKey

	// reportedFingerprint es la MISMA huella, anotada para INFORMAR en vez de
	// para escribir, y la separación no es cosmética (spec 19 §5.1).
	//
	// `stepFingerprint` gobierna dos escrituras —el registro y la entrada de
	// índice— y por eso el camino del salto no la anota: hacerlo reescribiría la
	// entrada del step revivido y le refrescaría el TTL, que es justo lo que la
	// spec 09 §9.4 evitó a propósito. Pero un `step_finished{from_cache: true}`
	// sin huella no deja comparar contra qué revivió, así que el dato hace falta
	// igual. Un solo campo para los dos usos sería un salto que se auto-renueva.
	reportedFingerprint cache.CacheKey

	// evidence es el registro que estuvo VIGENTE para este step, y viaja en las
	// DOS mitades (spec 19 §5.1, 28 §5.3):
	//
	//   - cuando el step revivió, el registro que lo revivió — la respuesta a
	//     «¿cuándo se probó esto por última vez?», que es la afirmación de valor
	//     del motor;
	//   - cuando el step ejecutó, el que acaba de escribir. Sin esta mitad, un
	//     rollback anclado a una ejecución pasada tendría que derivar por fechas
	//     qué registro estuvo vigente, que es guardar una conclusión.
	//
	// El valor cero es legítimo y frecuente: los tres casos que ejecutan y no
	// escriben registro —sin `config.yaml`, sin comandos, sin `rules`— no invocan
	// ninguno.
	evidence EvidenceFact

	// reason es POR QUÉ el step terminó como terminó. Lo anota quien DECIDE —el
	// handler 03— y lo emite quien PRESENCIA el cierre, que es el ejecutable: es
	// la frontera que la spec 19 §5.1' declara para que no se implemente al revés.
	//
	// Hasta aquí los seis motivos eran constantes privadas del handler y viajaban
	// por el log como frases, o sea texto libre descartable por diseño.
	reason command.StepReason

	// stepConfig es lo que el step declara sobre sí mismo en su `config.yaml`
	// (spec 13). Vive aquí por la misma razón que la huella: es estado de ESTA
	// cadena y muere con ella.
	//
	// Lo carga el handler 03 justo antes de decidir, que es el primer momento en
	// que hace falta. Hasta entonces vale `NoStepConfig()`, y esa NO es una
	// suposición peligrosa: significa «no declara ámbito», y de ahí se sigue que
	// no se escribe nada — la dirección segura del olvido, igual que
	// `OriginDeclared` es el valor cero de `Origin`.
	stepConfig StepConfig

	// sourcedDeclarations son las variables que este step declara CON FUENTE
	// (spec 14 §5.2): las que dicen `resolve: step-output` o `resolve: state`.
	//
	// Vive aquí por la misma razón que la huella y el `config.yaml`: es estado de
	// ESTA cadena y muere con ella. Y es lo que permite que el material de
	// identidad lleve la DECLARACIÓN de estas variables en vez de su valor
	// resuelto (§5.3), que es la regla que gobierna el resto del catálogo.
	//
	// Los literales NO están aquí: los suyos ya viajan enteros dentro del mapa
	// acumulado, porque su declaración y su valor son la misma cosa.
	sourcedDeclarations []VariableDeclaration

	// executed distingue «este step ejecutó sus comandos» de «este step
	// revivió». Los dos terminan en éxito y sólo uno es un hecho nuevo.
	//
	// La distinción no es contable: si revivir escribiera registro, cada corrida
	// añadiría uno idéntico bajo la misma clave —y con la huella SIN anotar,
	// porque el camino de revivir no la anota—, así que el step dejaría de
	// revivir para siempre. Es el observable que fija
	// `TestRunCommand_ReejecucionSinCambios` a partir de la tercera corrida.
	executed bool
}

func NewStepRequestHandler(executionContext *command.ExecutionContext, stepName string) *StepRequestHandler {

	requestHandler := &StepRequestHandler{
		stepName:         stepName,
		stepStatus:       command.StepFailure,
		skipReason:       SkipReasonNone,
		stepConfig:       NoStepConfig(),
		executionContext: executionContext,
	}
	return requestHandler
}

func (rh *StepRequestHandler) Execute() error {
	return rh.executionContext.CommandExecutable().Execute(rh.executionContext)
}

func (rh *StepRequestHandler) ProjectUrl() string {
	return rh.executionContext.ProjectUrl()
}

func (rh *StepRequestHandler) PipelineUrl() string {
	return rh.executionContext.PipelineUrl()
}

func (rh *StepRequestHandler) AddAccumulatedVars(variable command.Variable) {
	rh.executionContext.AccumulatedVars().Add(variable)
}

func (rh *StepRequestHandler) AddAccumulatedVarsAll(variables *command.ExecutionVariableMap) {
	for _, variable := range *variables {
		rh.executionContext.AccumulatedVars().Add(variable)
	}
}

func (rh *StepRequestHandler) AddCommand(command command.Command) {
	rh.executionContext.SetCommand(command)
}

func (rh *StepRequestHandler) AccumulatedVars() *command.ExecutionVariableMap {
	return rh.executionContext.AccumulatedVars()
}

// SetSourcedDeclarations y SourcedDeclarations transportan lo que el step declara
// con `resolve` desde el handler que lo lee hasta el material de identidad.
func (rh *StepRequestHandler) SetSourcedDeclarations(declarations []VariableDeclaration) {
	rh.sourcedDeclarations = declarations
}

func (rh *StepRequestHandler) SourcedDeclarations() []VariableDeclaration {
	return rh.sourcedDeclarations
}

// ProducedVars son las variables que ESTE step extrajo del stdout de sus
// comandos: lo que produjo, frente a todo lo que consumió (spec 14 §6).
func (rh *StepRequestHandler) ProducedVars() *command.ExecutionVariableMap {
	return rh.executionContext.ProducedVars()
}

func (rh *StepRequestHandler) StartedAt() time.Time {
	return rh.executionContext.StartedAt()
}

func (rh *StepRequestHandler) Environment() string {
	return rh.executionContext.Environment()
}

func (rh *StepRequestHandler) ProjectStatus() string {
	return rh.executionContext.ProjectStatus()
}

func (rh *StepRequestHandler) Ctx() *context.Context {
	return rh.executionContext.Ctx()
}

func (rh *StepRequestHandler) MarkStepSuccess() {
	rh.stepStatus = command.StepSuccess
}

// MarkStepCached registra que el step REVIVIÓ: terminó bien sin ejecutar ni un
// comando.
//
// Hasta la spec 19 este camino se marcaba `SUCCESS`, y no pasaba nada porque
// nadie leía el status. Ahora alimenta el `status` de `step_finished` (BL-4), y
// «se ejecutó correctamente» sobre cero comandos sería una conclusión y no un
// hecho — la misma razón por la que un step sin comandos es `SKIPPED` y no
// `SUCCESS` (spec 04 §5.3).
func (rh *StepRequestHandler) MarkStepCached() {
	rh.stepStatus = command.StepCached
}

// MarkStepExecuted registra que el step va a ejecutar sus comandos, y con ello
// que dejará un registro si termina bien. Lo llama el handler 04 justo antes del
// primer comando: un step que revive no pasa por aquí.
func (rh *StepRequestHandler) MarkStepExecuted() {
	rh.executed = true
}

// WasExecuted dice si hubo ejecución real. Sólo entonces hay un hecho que
// registrar (spec 11 §5.3: «uno por ejecución real del step — nunca cuando se
// revive»).
func (rh *StepRequestHandler) WasExecuted() bool {
	return rh.executed
}

// MarkStepSkipped registra que el step no se ejecutó, y por qué. Es un resultado
// propio: ni SUCCESS —que afirmaría una ejecución que no hubo— ni FAILURE.
//
// La razón se traduce AQUÍ al vocabulario del registro, con un `switch` y no con
// una conversión de cadena: los dos vocabularios coinciden hoy en su único valor
// y no tienen por qué seguir coincidiendo, y una conversión silenciosa haría que
// añadir un `SkipReason` emitiera un motivo que el registro no conoce.
func (rh *StepRequestHandler) MarkStepSkipped(reason SkipReason) {
	rh.stepStatus = command.StepSkipped
	rh.skipReason = reason

	switch reason {
	case SkipReasonNoCommands:
		rh.reason = command.ReasonNoCommands
	default:
		rh.reason = command.ReasonNone
	}
}

// RecordStepFingerprint anota qué huella se escribirá en el registro DESPUÉS de
// que el step termine bien (spec 09 §5.2, heredado por la 10 y por la 11).
//
// Anotar no es escribir: si el step falla —o si el proceso muere a mitad— esto
// se pierde con la cadena, que es exactamente lo que se quiere. Un registro sólo
// debe existir para steps que terminaron.
func (rh *StepRequestHandler) RecordStepFingerprint(fingerprint cache.CacheKey) {
	rh.stepFingerprint = fingerprint
}

// StepFingerprint es la huella anotada en su forma canónica, o la cadena vacía
// si no se pudo componer el material del step.
//
// Vacía NO es un error aquí: el step se ejecutó igual y lo que produjo es estado
// real que hay que guardar. Lo que un registro sin huella no puede es revivir
// (ver `state.StepRecord.Revives`), que es exactamente la asimetría de la
// spec 11 §4 — ante la duda, guardar de más.
func (rh *StepRequestHandler) StepFingerprint() string {
	return rh.stepFingerprint.String()
}

// ReportStepFingerprint anota la huella para INFORMAR: la que el hecho de cierre
// del step transporta, se vaya a escribir o no (ver `reportedFingerprint`).
//
// Se llama en los DOS caminos —el que ejecuta y el que revive—, que es lo que
// `RecordStepFingerprint` no puede hacer sin refrescarle el TTL al step que
// revivió.
func (rh *StepRequestHandler) ReportStepFingerprint(fingerprint cache.CacheKey) {
	rh.reportedFingerprint = fingerprint
}

// ReportedFingerprint es la huella anotada para informar, o la cadena vacía.
//
// Vacía es legítima por dos vías distintas y ninguna es un error: no se pudo
// componer el material (spec 11 §4), o el step no declara `state_changed` y por
// tanto no compara contenidos (spec 15 §5.4).
func (rh *StepRequestHandler) ReportedFingerprint() string {
	return rh.reportedFingerprint.String()
}

// RecordEvidence y Evidence transportan el registro que estuvo vigente para este
// step desde quien lo conoce hasta quien emite el cierre. Ver `evidence`.
func (rh *StepRequestHandler) RecordEvidence(evidence EvidenceFact) {
	rh.evidence = evidence
}

func (rh *StepRequestHandler) Evidence() EvidenceFact {
	return rh.evidence
}

// RecordReason y Reason transportan POR QUÉ este step terminó como terminó,
// desde el handler que lo DECIDE hasta el ejecutable que lo EMITE.
func (rh *StepRequestHandler) RecordReason(reason command.StepReason) {
	rh.reason = reason
}

func (rh *StepRequestHandler) Reason() command.StepReason {
	return rh.reason
}

// StateScope es el ámbito bajo el que este step se recuerda, o el ámbito cero si
// no declara ninguno.
//
// Es `StateKey().Scope()` sin el error, y existe porque el HECHO no puede fallar
// por no poder componer una clave: el ámbito viaja en `step_finished` como campo
// —no dentro de un hash— para que un consumidor pueda filtrar por él, y no
// saberlo es una ausencia legítima, no un motivo para no emitir el cierre.
func (rh *StepRequestHandler) StateScope() state.Scope {
	key, declared, err := rh.StateKey()
	if err != nil || !declared {
		return state.Scope{}
	}
	return key.Scope()
}

// IndexKey devuelve la clave con la que se indexa el registro. La segunda salida
// es falsa cuando no hay nada que indexar: el step revivió, o no se pudo
// componer su material.
func (rh *StepRequestHandler) IndexKey() (cache.CacheKey, bool) {
	return rh.stepFingerprint, !rh.stepFingerprint.IsZero()
}

// SetStepConfig y StepConfig transportan lo que el step declara sobre sí mismo
// desde el handler que lo lee hasta el `StepExecutable` que persiste el
// resultado.
func (rh *StepRequestHandler) SetStepConfig(stepConfig StepConfig) {
	rh.stepConfig = stepConfig
}

func (rh *StepRequestHandler) StepConfig() StepConfig {
	return rh.stepConfig
}

// StateKey es la clave de posición del ámbito que el step DECLARA: el único
// sitio donde escribe, y el que consulta para saber si ya se ejecutó aquí
// (spec 13 §5.4).
//
// La segunda salida es FALSA cuando el step no tiene `config.yaml`. No es un
// error ni una clave degradada: es un step que no declara ámbito, y por tanto
// uno que se ejecuta siempre y no persiste registro (§5.3). Inventarle uno
// —`environment` por defecto, digamos— sería la misma deducción implícita que
// esta spec retira, sólo que en otro archivo.
//
// Hasta la spec 13 aquí había DOS claves y la decisión de re-ejecutar consultaba
// SIEMPRE la del ambiente. Con el ámbito declarado queda una, y para un step
// `scope: project` es otra distinta de la que se consultaba antes.
func (rh *StepRequestHandler) StateKey() (state.Key, bool, error) {
	if !rh.stepConfig.IsDeclared() {
		return state.Key{}, false, nil
	}
	scope, err := rh.stepConfig.Scope().StateScope(rh.Environment())
	if err != nil {
		return state.Key{}, false, err
	}
	key, err := state.NewKey(rh.ProjectUrl(), scope, rh.StepFullName())
	if err != nil {
		return state.Key{}, false, err
	}
	return key, true, nil
}

// ProjectStateKey y EnvironmentStateKey son las dos claves que la CARGA
// consulta, y sólo la carga: un step lee los dos ámbitos y escribe en uno
// (spec 13 §5.4). Que sigan siendo dos no es el residuo que la spec 13 borra
// —ése era escribir dos registros— sino la asimetría que la spec establece.
func (rh *StepRequestHandler) ProjectStateKey() (state.Key, error) {
	return state.NewKey(rh.ProjectUrl(), state.NewProjectScope(), rh.StepFullName())
}

func (rh *StepRequestHandler) EnvironmentStateKey() (state.Key, error) {
	scope, err := state.NewEnvironmentScope(rh.Environment())
	if err != nil {
		return state.Key{}, err
	}
	return state.NewKey(rh.ProjectUrl(), scope, rh.StepFullName())
}

func (rh *StepRequestHandler) StepStatus() command.StepStatus {
	return rh.stepStatus
}

func (rh *StepRequestHandler) SkipReason() SkipReason {
	return rh.skipReason
}

func (rh *StepRequestHandler) WasSkipped() bool {
	return rh.stepStatus == command.StepSkipped
}

func (rh *StepRequestHandler) StepName() string {
	return rh.executionContext.StepName()
}

func (rh *StepRequestHandler) StepNameExe() string {
	return rh.executionContext.Step().Name()
}

func (rh *StepRequestHandler) StepFullName() string {
	return rh.executionContext.StepFullName()
}

func (rh *StepRequestHandler) SetStepName(stepName command.StepName) {
	rh.executionContext.SetStepName(stepName)
}

func (rh *StepRequestHandler) Emit(line string) {
	rh.executionContext.Emit(line)
}

// AQUÍ vivía `PipelineLocalPath()`, y con él la última razón por la que la
// cadena de step conocía el disco. La retira la spec 18 §5.2: el material del
// step llega cargado por el resolutor de la cadena de pipeline, así que ningún
// handler de aquí compone ya una ruta del pipelinecode.
