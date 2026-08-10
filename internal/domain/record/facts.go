package record

import (
	"context"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/step"
)

// Facts traduce lo que las cadenas de STEP y de COMANDO observan a hechos del
// registro.
//
// Es el adaptador de los dos puertos —`step.FactSink` y `command.FactSink`— y
// vive aquí y no en ellos por la dirección de la dependencia: `record` importa
// `command` para transportar sus status sin duplicarlos y `deployment` para la
// evidencia, y `deployment` importa `step`. Las dos cadenas declaran el puerto
// donde lo consumen; el implementador tiene que estar de este lado.
//
// Es UN tipo y no dos porque los tres hechos que traduce salen del mismo emisor
// y con la misma numeración: `seq` es la posición dentro del INTENTO, no dentro
// de una cadena, y dos adaptadores invitarían a darle dos emisores.
//
// Lo único que hace además de reenviar es lo que exige el vocabulario cerrado:
// CLASIFICAR el error, derivar el exit code y resumir el valor. Traducir eso en
// el llamador obligaría a las cadenas a conocer el vocabulario del registro y
// dejaría tres traducciones del mismo error.
type Facts struct {
	emitter *Emitter
}

var (
	_ step.FactSink    = (*Facts)(nil)
	_ command.FactSink = (*Facts)(nil)
)

func NewFacts(emitter *Emitter) *Facts {
	return &Facts{emitter: emitter}
}

// ── Cadena de step ──────────────────────────────────────────────────────────

func (f *Facts) StepStarted(ctx *context.Context, fact step.StepStartedFact) error {
	return f.emitter.Emit(ctx, StepStarted{StepID: fact.StepID})
}

func (f *Facts) StepFinished(ctx *context.Context, fact step.StepFinishedFact) error {
	payload := StepFinished{
		StepID:          fact.StepID,
		Scope:           fact.Scope,
		Status:          fact.Status,
		Duration:        fact.Duration,
		FromCache:       fact.FromCache,
		Reason:          fact.Reason,
		StepFingerprint: fact.StepFingerprint,
		Evidence:        evidenceOf(fact.Evidence),
		ErrorClass:      ClassifyError(fact.Err),
	}
	// El exit code es un PUNTERO porque su ausencia significa algo: un step
	// exitoso, revivido o saltado no tiene ninguno que reportar, y un `0` diría lo
	// contrario. Tampoco lo tiene un fallo que no fue de un comando.
	if code, ok := ExitCodeOf(fact.Err); ok {
		payload.ExitCode = &code
	}
	return f.emitter.Emit(ctx, payload)
}

// evidenceOf compone la referencia al registro vigente.
//
// `Deployment` y `Attempt` se quedan en cero, y es una verdad y no un hueco:
// `state.Provenance` es hoy `{ExecutionID, At}` y no sabe de qué despliegue
// salió el registro. Rellenarlos con los del despliegue EN CURSO afirmaría que
// el registro salió de éste, que es lo contrario de lo que la evidencia dice.
func evidenceOf(fact step.EvidenceFact) EvidenceRef {
	if fact.IsZero() {
		return EvidenceRef{}
	}
	return EvidenceRef{
		ExecutionID: fact.ExecutionID,
		At:          fact.At,
		StateKey:    fact.StateKey,
		RecordID:    fact.RecordID,
	}
}

// ── Cadena de comando ───────────────────────────────────────────────────────

func (f *Facts) CommandStarted(ctx *context.Context, fact command.CommandStartedFact) error {
	return f.emitter.Emit(ctx, CommandStarted{
		StepID:      fact.StepID,
		CommandName: fact.CommandName,
	})
}

func (f *Facts) CommandFinished(ctx *context.Context, fact command.CommandFinishedFact) error {
	return f.emitter.Emit(ctx, CommandFinished{
		StepID:      fact.StepID,
		CommandName: fact.CommandName,
		Status:      fact.Status,
		Duration:    fact.Duration,
		ExitCode:    fact.ExitCode,
		ErrorClass:  ClassifyError(fact.Err),
	})
}

// ── Los dos dueños de parameter_resolved ────────────────────────────────────

func (f *Facts) ParameterResolved(ctx *context.Context, fact command.ParameterFact) error {
	return f.emitter.Emit(ctx, ParameterResolved{
		Name:   fact.Name,
		Source: fact.Source,
		Digest: DigestOf(fact.Value),
	})
}
