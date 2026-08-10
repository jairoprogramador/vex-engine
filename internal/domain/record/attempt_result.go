package record

import (
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	"github.com/jairoprogramador/vex-engine/internal/domain/deployment"
	"github.com/jairoprogramador/vex-engine/internal/domain/state"
)

// AttemptStatus es el desenlace de un intento.
//
// Son los tres de `command.ExecutionStatus` más uno que aquel enum no puede
// tener, y esa asimetría es el motivo de que exista este tipo: `interrupted` no
// es un estado que nadie ALCANCE, es lo que se deduce de que nadie llegara a
// escribir el desenlace. Quien está vivo para marcarlo, por definición, no fue
// interrumpido.
type AttemptStatus string

const (
	AttemptSucceeded AttemptStatus = "succeeded"
	AttemptFailed    AttemptStatus = "failed"

	// AttemptCancelled es la parada pedida: `SIGINT`/`SIGTERM` con tiempo de
	// desenrollar las cadenas (spec 07). Se escribe, así que se distingue.
	AttemptCancelled AttemptStatus = "canceled"

	// AttemptInterrupted es la muerte dura: el proceso se fue sin cerrar el
	// intento. NO se emite nunca — se DERIVA de la ausencia de
	// `attempt_finished`, y es el caso que justifica el modelo entero.
	AttemptInterrupted AttemptStatus = "interrupted"
)

func (s AttemptStatus) String() string { return string(s) }

// StepResult es lo que le pasó a un step dentro del intento, plegado de sus dos
// hechos.
type StepResult struct {
	StepID string
	Scope  state.Scope

	// Status es el terminal que trajo su `step_finished`, o `RUNNING` si nunca
	// llegó: un step abierto y no cerrado es exactamente eso, y decir «falló»
	// sería una conclusión que nadie observó.
	Status command.StepStatus

	// Finished dice si llegó su `step_finished`. Es lo que separa «no terminó» de
	// «terminó mal», que son dos hechos distintos y sólo uno es un fallo.
	Finished bool

	Duration  time.Duration
	FromCache bool

	// Reason es por qué terminó así: qué regla dijo que sí, o que ninguna lo
	// dijo. Junto a `FromCache` es lo que separa un caché que funciona de una
	// configuración que revive basura (spec 19).
	Reason command.StepReason

	StepFingerprint string
	Evidence        EvidenceRef
	ExitCode        *int
	ErrorClass      ErrorClass
}

// AttemptResult es lo que un intento fue, DERIVADO de sus hechos.
//
// No se guarda en ninguna parte: guardarlo sería duplicar lo que `Fold` deriva,
// y dos fuentes de verdad para el mismo dato divergen. Por eso se descarta
// Memento (spec 17 §5.3').
type AttemptResult struct {
	// Deployment y Attempt salen de `attempt_started` y del sobre. Van en cero si
	// los hechos empiezan por la mitad —un archivo truncado por delante—, que es
	// una posibilidad real y no un caso teórico.
	Deployment deployment.DeploymentID
	Attempt    deployment.Attempt

	Status AttemptStatus

	Actor  string
	Runner string

	StartedAt  time.Time
	FinishedAt time.Time

	// Duration es el tiempo entre los dos instantes, y sólo existe si los dos
	// existen. Un intento interrumpido no tiene duración: tiene un principio.
	Duration time.Duration

	// Steps son los steps alcanzados, en el orden en que se abrieron.
	Steps []StepResult

	// LastStep es hasta dónde se llegó. **Es lo que hace útil un intento
	// interrumpido**, y por eso es un campo propio y no algo que el consumidor
	// tenga que buscar en la lista.
	LastStep string

	// ExitCode y ErrorClass son los del PRIMER step que falló, que es el que
	// tumbó el intento: los posteriores no llegaron a correr.
	ExitCode   *int
	ErrorClass ErrorClass

	// Commands es cuántos comandos se cerraron, y Failed cuántos fallaron. Son
	// conteos, no una lista: el detalle por comando está en los hechos, y
	// duplicarlo aquí sería la misma divergencia que se evita con el resultado.
	Commands       int
	FailedCommands int

	// Artifacts y StaleClones son los otros dos hechos que un consumidor cuenta
	// sin tener que releer la tira entera.
	Artifacts   int
	StaleClones int
}

// IsTerminal dice si el intento cerró por su propio pie.
func (r AttemptResult) IsTerminal() bool {
	switch r.Status {
	case AttemptSucceeded, AttemptFailed, AttemptCancelled:
		return true
	default:
		return false
	}
}

// IsValidTarget dice si este intento puede NOMBRARSE como destino: terminó bien
// y todos sus steps alcanzaron un terminal correcto.
//
// Existe aquí, en el modelo de lectura, porque es derivación pura y porque el
// listado que la spec 22 expone tiene que MARCARLO: quien elige una ejecución
// pasada para volver a ella (spec 28 §5.2) necesita saber antes de elegir que
// ese intento sirve. Sin la marca, el usuario escoge uno y falla después, que es
// el modo de fallo que la consulta existe para evitar.
//
// Un step que revivió o que se saltó cuenta como correcto: no ejecutó nada, pero
// su resultado es el que estaba vigente y eso es exactamente lo que un destino
// declara. Lo que descalifica es un fallo, o un par que quedó ABIERTO —un step
// que empezó y del que nadie escribió el cierre—, porque de ése no se sabe si
// llegó a hacer lo que dice.
func (r AttemptResult) IsValidTarget() bool {
	if r.Status != AttemptSucceeded {
		return false
	}
	if len(r.Steps) == 0 {
		return false
	}
	for _, step := range r.Steps {
		if !step.Finished {
			return false
		}
		if step.Status == command.StepFailure {
			return false
		}
	}
	return true
}

// Step busca el resultado de un step por su identificador.
func (r AttemptResult) Step(stepID string) (StepResult, bool) {
	for _, step := range r.Steps {
		if step.StepID == stepID {
			return step, true
		}
	}
	return StepResult{}, false
}
