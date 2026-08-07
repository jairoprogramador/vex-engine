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
func (rh *StepRequestHandler) MarkStepSkipped(reason SkipReason) {
	rh.stepStatus = command.StepSkipped
	rh.skipReason = reason
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

func (rh *StepRequestHandler) PipelineLocalPath() string {
	return rh.executionContext.PipelineLocalPath()
}
