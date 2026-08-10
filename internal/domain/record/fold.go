package record

import (
	"slices"
	"sort"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

// Fold deriva el resultado de un intento a partir de sus hechos.
//
// # Es una función de dominio PURA, no un servicio
//
// Sin estado, sin dependencias, determinista. Que se pueda ejecutar idéntica en
// el backend (spec 26) es CONSECUENCIA de eso, no una decisión aparte: misma
// lógica, dos lugares, y por eso vive en dominio puro sin una sola dependencia
// de I/O.
//
// No muta el slice que recibe. Lo clona antes de ordenar, porque un plegado que
// reordena los hechos de quien lo llama no es puro por mucho que lo parezca.
//
// # Ordena por `seq`, NUNCA por `time`
//
// El reloj no es confiable —contenedor, máquina remota, ajuste NTP— y el orden
// sí importa para plegar. El orden es estable: dos hechos con la misma posición
// conservan el orden en que llegaron, que es lo único razonable cuando la
// posición miente.
//
// # El caso que justifica el modelo entero
//
// Si no llega `attempt_finished`, el resultado IGUAL EXISTE y dice exactamente
// hasta dónde se llegó: `interrupted`, con el último step alcanzado. Es lo que
// hace que un `Ctrl-C` o una Fly Machine que muere dejen datos útiles en vez de
// nada.
//
// # Y tolera lo que no entiende
//
// Un tipo de evento desconocido —escrito por un motor más nuevo— se ignora al
// plegar en vez de hacer fallar el pliegue. Es la mitad lectora del OCP: añadir
// un tipo no toca a `Fold` para los existentes, y encontrarse uno no invalida
// los que sí se entienden.
func Fold(events []Event) AttemptResult {
	ordered := slices.Clone(events)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Seq().Before(ordered[j].Seq())
	})

	result := AttemptResult{}
	index := make(map[string]int, len(ordered))
	finished := false

	for _, event := range ordered {
		if event.IsZero() {
			continue
		}
		if result.Attempt.IsZero() {
			result.Attempt = event.Attempt()
		}

		switch payload := event.Payload().(type) {
		case AttemptStarted:
			result.Deployment = payload.Deployment
			result.Actor = payload.Actor
			result.Runner = payload.Runner
			result.StartedAt = event.At()
			result.Attempt = event.Attempt()

		case StepStarted:
			position, seen := index[payload.StepID]
			if !seen {
				position = len(result.Steps)
				index[payload.StepID] = position
				result.Steps = append(result.Steps, StepResult{StepID: payload.StepID})
			}
			step := &result.Steps[position]
			step.Scope = payload.Scope
			step.StepFingerprint = payload.StepFingerprint
			// `RUNNING` y no un terminal: mientras no llegue su cierre, lo único
			// observado es que empezó.
			step.Status = command.StepRunning
			result.LastStep = payload.StepID

		case StepFinished:
			position, seen := index[payload.StepID]
			if !seen {
				// Un cierre sin apertura no se descarta: el hecho ocurrió, y un
				// archivo truncado por delante es exactamente cómo se ve una
				// máquina efímera que murió y dejó la cola.
				position = len(result.Steps)
				index[payload.StepID] = position
				result.Steps = append(result.Steps, StepResult{StepID: payload.StepID})
			}
			step := &result.Steps[position]
			step.Status = payload.Status
			step.Finished = true
			step.Duration = payload.Duration
			step.FromCache = payload.FromCache
			step.Reason = payload.Reason
			step.Evidence = payload.Evidence
			step.ExitCode = payload.ExitCode
			step.ErrorClass = payload.ErrorClass
			result.LastStep = payload.StepID

			// El ámbito y la huella los pisa el cierre SÓLO si los trae: es aquí
			// donde el dato existe en este motor (ver `StepStarted`), y un cierre
			// que no los conoce no debe borrar los que la apertura sí trajo.
			if !payload.Scope.IsZero() {
				step.Scope = payload.Scope
			}
			if payload.StepFingerprint != "" {
				step.StepFingerprint = payload.StepFingerprint
			}

			// El fallo del intento es el del PRIMER step que falló: los
			// posteriores no llegaron a correr, así que atribuirle el último
			// sería contar el desenlace al revés.
			if payload.Status == command.StepFailure && result.ErrorClass.IsZero() && result.ExitCode == nil {
				result.ExitCode = payload.ExitCode
				result.ErrorClass = payload.ErrorClass
			}

		case CommandFinished:
			result.Commands++
			if payload.Status == command.CommandFailure {
				result.FailedCommands++
			}

		case ArtifactProduced:
			result.Artifacts++

		case StaleCloneUsed:
			result.StaleClones++

		case AttemptFinished:
			result.Status = payload.Status
			result.FinishedAt = event.At()
			finished = true
		}
	}

	if !finished {
		result.Status = AttemptInterrupted
	}
	if !result.StartedAt.IsZero() && !result.FinishedAt.IsZero() {
		result.Duration = result.FinishedAt.Sub(result.StartedAt)
	}
	return result
}
