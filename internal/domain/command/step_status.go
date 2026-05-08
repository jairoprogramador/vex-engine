package command

type StepStatus string

const (
	StepSuccess    StepStatus = "SUCCESS"
	StepFailure    StepStatus = "FAILURE"
	StepCached     StepStatus = "CACHED"
	StepRegistered StepStatus = "REGISTERED"
	StepRunning    StepStatus = "RUNNING"
)

func (s StepStatus) String() string {
	return string(s)
}

func (s StepStatus) IsTerminal() bool {
	return s == StepSuccess || s == StepFailure || s == StepCached
}
