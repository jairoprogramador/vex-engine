package status

import (
	"fmt"
	"time"
)

const (
	TimeRuleName     = "time_rule"
	CurrentTimeParam = "current_time"
)

const defaultTTLDuration = 30 * 24 * time.Hour

// evidenceTimeLayout es la forma en la que un instante viaja dentro de una
// `Evidence`, que transporta cadenas. RFC3339Nano porque es reversible: lo que
// el `StatusWriter` vuelve a convertir en `time.Time` es exactamente el instante
// que la regla comparó.
const evidenceTimeLayout = time.RFC3339Nano

// FormatEvidenceTime y ParseEvidenceTime son los dos lados de la misma
// conversión y se mantienen juntos a propósito.
func FormatEvidenceTime(at time.Time) string {
	return at.UTC().Format(evidenceTimeLayout)
}

func ParseEvidenceTime(value string) (time.Time, error) {
	at, err := time.Parse(evidenceTimeLayout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("interpretar el instante %q de la evidencia: %w", value, err)
	}
	return at, nil
}

type TimeRule struct {
	repository TimeStatusRepository
}

func NewTimeRule(repository TimeStatusRepository) TimeRule {
	return TimeRule{repository: repository}
}

func (e TimeRule) Name() string { return TimeRuleName }

// Evaluate comprueba si expiró el TTL de 30 días y NO escribe (spec 09 §5.1).
//
// Dos correcciones de transición respecto de lo que hacía antes, ambas de la
// spec 09 §5.4:
//
//   - Los mensajes estaban INVERTIDOS: el `Skip` decía «el tiempo de codigo a
//     expirado» y el `Run` decía «el tiempo a expirado». Decían lo mismo para
//     los dos casos contrarios.
//   - La marca se refrescaba solo cuando ESTA regla decidía ejecutar. Un step
//     re-ejecutado porque cambió el código no reiniciaba su TTL, así que el TTL
//     medía algo que nadie quiso medir. Ahora la evidencia lleva SIEMPRE el
//     instante actual: quien la persiste es el camino de éxito del step, de modo
//     que la marca se refresca cuando el step se ejecuta, por el motivo que sea.
//
// El instante viene del puerto `shared.Clock` a través de `CurrentTimeParam`
// —es el `startedAt` del agregado (spec 07)—, así que el valor con el que se
// compara y el que se persiste son el mismo, y los dos son fijables desde un
// test: los dos lados del borde de 30 días sin esperar treinta días.
//
// La regla entera desaparece en la spec 10, cuando el TTL pase a ser metadato de
// expiración del caché.
func (e TimeRule) Evaluate(ctx RuleContext) (Decision, []Evidence, error) {
	currentTime, err := GetParam[time.Time](ctx, CurrentTimeParam)
	if err != nil {
		return e.sinObservar("no se pudo obtener el instante actual", err)
	}

	projectUrl, err := GetParam[string](ctx, ProjectUrlParam)
	if err != nil {
		return e.sinObservar("no se pudo obtener la url del proyecto", err)
	}

	environment, err := GetParam[string](ctx, EnvironmentParam)
	if err != nil {
		return e.sinObservar("no se pudo obtener el ambiente de ejecución", err)
	}

	step, err := GetParam[string](ctx, StepParam)
	if err != nil {
		return e.sinObservar("no se pudo obtener el paso de ejecución", err)
	}

	current := FormatEvidenceTime(currentTime)

	previousTime, err := e.repository.Get(projectUrl, environment, step)
	if err != nil {
		return DecisionUndetermined("no se pudo leer el instante de la ejecución anterior"),
			[]Evidence{NewEvidence(TimeRuleName, current, "", false)},
			err
	}

	previous := FormatEvidenceTime(previousTime)
	expiration := previousTime.Add(defaultTTLDuration)

	if currentTime.Before(expiration) {
		return DecisionSkip("no ha expirado el tiempo desde la última ejecución"),
			[]Evidence{NewEvidence(TimeRuleName, current, previous, false)},
			nil
	}
	return DecisionRun("ha expirado el tiempo desde la última ejecución"),
		[]Evidence{NewEvidence(TimeRuleName, current, previous, true)},
		nil
}

func (e TimeRule) sinObservar(reason string, err error) (Decision, []Evidence, error) {
	return DecisionUndetermined(reason), []Evidence{NoEvidence(TimeRuleName)}, err
}
