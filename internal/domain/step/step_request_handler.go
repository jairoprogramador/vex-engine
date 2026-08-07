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

// ProjectStateKey y EnvironmentStateKey son las DOS claves de posición bajo las
// que este step recuerda lo que dejó.
//
// Son dos, y no una, sólo hasta la spec 13: mientras el ámbito no lo declare el
// step, un mismo step puede producir variables comunes al proyecto —las que hoy
// se marcan `shared`— y variables propias del ambiente, y las primeras tienen
// que seguir siendo visibles desde cualquier ambiente. Cuando el step declare UN
// ámbito, una de las dos desaparece y con ella la bifurcación entera.
//
// La decisión de re-ejecutar consulta SIEMPRE la del ambiente: es la que
// responde «¿este step ya se ejecutó AQUÍ?».
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
