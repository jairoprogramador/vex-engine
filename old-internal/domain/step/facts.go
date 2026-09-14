package step

import (
	"context"
	"time"

	"github.com/jairoprogramador/vex-engine/old-internal/domain/command"
	"github.com/jairoprogramador/vex-engine/old-internal/domain/state"
)

// FactSink es por donde la cadena de STEP publica lo que observa.
//
// # Por qué es un puerto y no el emisor de `record` directamente
//
// Es la misma razón de dirección que en `command.FactSink`, sólo que aquí es
// insalvable: `record` importa `deployment` —la evidencia lleva el despliegue
// del que salió el registro— y `deployment` importa `step`, porque el material
// de cada step del objeto lleva su `StepConfig`. Un import de `step` a `record`
// cerraría el ciclo.
//
// La consecuencia útil es que este puerto habla en los tipos que `step` YA
// conoce —`state.Key`, `state.RecordID`, `command.StepStatus`— y el adaptador
// compone con ellos el hecho del registro. Quien presencia entrega lo que vio;
// el vocabulario cerrado del registro se aplica del otro lado.
type FactSink interface {
	StepStarted(ctx *context.Context, fact StepStartedFact) error
	StepFinished(ctx *context.Context, fact StepFinishedFact) error

	// ParameterResolved es el mismo hecho que emite la cadena de comando, con el
	// mismo tipo: los dos dueños de `parameter_resolved` (spec 19 §5.1) son los
	// resolutores de declaraciones —que están en esta cadena y emiten en la CARGA
	// del step— y el extractor de salidas, que emite lo que NACE al ejecutar.
	// Dos momentos, un solo hecho.
	ParameterResolved(ctx *context.Context, fact command.ParameterFact) error
}

// StepStartedFact abre un step.
//
// Lleva sólo el identificador, y es un HALLAZGO de la implementación que merece
// estar escrito: el ámbito y la huella se conocen DENTRO de la cadena —el
// primero lo anota el handler 03 al leer el `config.yaml`, la segunda la compone
// su `decide`— y este hecho se emite ANTES de entrar en ella, que es lo que hace
// que un `before` que falla no deje el par abierto. Los dos viajan en el cierre,
// que es donde el dato existe.
type StepStartedFact struct {
	StepID string
}

// StepFinishedFact cierra un step, y es el hecho con más carga del vocabulario.
type StepFinishedFact struct {
	StepID string
	Scope  state.Scope

	// Status es el terminal que el request ya calculaba y nadie leía (BL-4):
	// SUCCESS / FAILURE / CACHED / SKIPPED.
	Status command.StepStatus

	Duration time.Duration

	// FromCache dice que el step revivió: no ejecutó ni un comando.
	FromCache bool

	// Reason es qué regla dijo que sí, o que ninguna lo dijo. Sin él, `FromCache`
	// no distingue un caché que funciona de una configuración que revive basura.
	Reason command.StepReason

	// StepFingerprint es la huella con la que el step se comparó, en su forma
	// canónica con prefijo. Vacía es legítima por dos vías distintas.
	StepFingerprint string

	// Evidence es el registro que estuvo vigente, en las dos mitades: el que
	// revivió al step, o el que el step acaba de escribir.
	Evidence EvidenceFact

	// Err es el error tal cual se observó. El exit code y la clase los deriva el
	// adaptador, que es quien tiene el vocabulario cerrado.
	Err error
}

// EvidenceFact es la referencia al registro vigente en los términos del almacén.
//
// No lleva `deployment_id` ni `attempt` porque `state.Provenance` tampoco los
// lleva todavía: lo que se emite es exactamente lo que se sabe del registro, y
// rellenar esos dos con el despliegue EN CURSO diría que el registro salió de
// éste, que es justo lo contrario de lo que la evidencia afirma.
type EvidenceFact struct {
	ExecutionID string
	At          time.Time
	StateKey    state.Key
	RecordID    state.RecordID
}

// IsZero dice que este step no invoca ningún registro. Es lo normal en los tres
// casos que ejecutan sin poder recordarse: sin `config.yaml`, sin comandos y sin
// `rules`.
func (e EvidenceFact) IsZero() bool {
	return e.ExecutionID == "" && e.RecordID.IsZero() && e.StateKey.IsZero() && e.At.IsZero()
}

// NoFacts descarta lo que se le entrega.
//
// Existe para los tests de unidad de esta cadena, que miden lo que se PERSISTE
// —un registro por ejecución real, ninguno cuando se revive— y no lo que se
// emite. **No es el default de producción**: el motor escribe siempre en su área
// de trabajo, esté o no configurado un destino (§5.6), así que un `NoFacts`
// cableado en el factory sería un silencio que nadie pidió.
type NoFacts struct{}

var _ FactSink = NoFacts{}

func (NoFacts) StepStarted(*context.Context, StepStartedFact) error   { return nil }
func (NoFacts) StepFinished(*context.Context, StepFinishedFact) error { return nil }
func (NoFacts) ParameterResolved(*context.Context, command.ParameterFact) error {
	return nil
}
