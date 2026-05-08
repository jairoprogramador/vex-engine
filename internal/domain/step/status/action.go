package status

type Action int

const (
	ActionRun Action = iota
	ActionSkip
)

func (a Action) String() string {
	return []string{"run", "skip"}[a]
}
