package step

import (
	"context"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/cache"
	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type StepRequestHandler struct {
	executionContext *command.ExecutionContext
	stepName         string
	stepStatus       command.StepStatus
	skipReason       SkipReason

	// La clave bajo la que se escribirá la entrada, esperando a que el step
	// termine bien. Vive aquí —y no en el ExecutionContext— porque es estado de
	// ESTA cadena y muere con ella; el ExecutionContext ya es demasiado grande.
	//
	// Que muera con la cadena es la propiedad que se quiere, no un efecto
	// secundario: si el step no llega al final, la clave se pierde sin que nadie
	// tenga que borrarla.
	//
	// Sustituye al par {RuleContext, []Evidence} que la spec 09 tuvo que hacer
	// viajar hasta aquí. El contexto viajaba porque las cuatro claves eran
	// distintas y ninguna estaba en la evidencia; ahora la clave es una y ya
	// está calculada.
	cacheKey cache.CacheKey
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

// MarkStepSkipped registra que el step no se ejecutó, y por qué. Es un resultado
// propio: ni SUCCESS —que afirmaría una ejecución que no hubo— ni FAILURE.
func (rh *StepRequestHandler) MarkStepSkipped(reason SkipReason) {
	rh.stepStatus = command.StepSkipped
	rh.skipReason = reason
}

// RecordCacheKey anota bajo qué clave se escribirá la entrada DESPUÉS de que el
// step termine bien (spec 09 §5.2, heredado por la 10 §5.2).
//
// Anotar no es escribir: si el step falla —o si el proceso muere a mitad— esto
// se pierde con la cadena, que es exactamente lo que se quiere. Una entrada de
// caché sólo debe existir para pasos que terminaron.
func (rh *StepRequestHandler) RecordCacheKey(key cache.CacheKey) {
	rh.cacheKey = key
}

// CacheKey devuelve la clave anotada. La segunda salida es falsa cuando no hay
// nada que escribir: el step se saltó, o no se pudo componer su material.
func (rh *StepRequestHandler) CacheKey() (cache.CacheKey, bool) {
	return rh.cacheKey, !rh.cacheKey.IsZero()
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
