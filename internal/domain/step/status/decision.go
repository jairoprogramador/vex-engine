package status

type Decision struct {
	action Action
	reason string
}

func DecisionRun(reason string) Decision {
	return Decision{action: ActionRun, reason: reason}
}

func DecisionSkip(reason string) Decision {
	return Decision{
		action: ActionSkip,
		reason: reason,
	}
}

func (d Decision) ShouldRun() bool { return d.action == ActionRun }
func (d Decision) Reason() string  { return d.reason }
