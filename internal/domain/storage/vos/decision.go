package vos

import "time"

type Action int

const (
	ActionRun  Action = iota
	ActionSkip Action = iota
)

// Decision es la respuesta tipada de ExecutionDecider.Decide.
// Lleva la razón textual y, cuando la acción es Skip, el momento de la entrada
// histórica que justifica la omisión.
type Decision struct {
	action         Action
	reason         string
	matchedEntryAt time.Time
}

// DecisionRun construye una decisión de ejecutar el paso.
func DecisionRun(reason string) Decision {
	return Decision{action: ActionRun, reason: reason}
}

// DecisionSkip construye una decisión de omitir el paso porque existe una
// entrada histórica que satisface todas las reglas.
func DecisionSkip(matchedAt time.Time) Decision {
	return Decision{
		action:         ActionSkip,
		matchedEntryAt: matchedAt,
		reason:         "entrada histórica encontrada",
	}
}

func (d Decision) ShouldRun() bool      { return d.action == ActionRun }
func (d Decision) Reason() string       { return d.reason }
func (d Decision) MatchedAt() time.Time { return d.matchedEntryAt }
