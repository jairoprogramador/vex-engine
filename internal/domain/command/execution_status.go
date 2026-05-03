package command

type ExecutionStatus string

const (
	StatusQueued    ExecutionStatus = "queued"
	StatusRunning   ExecutionStatus = "running"
	StatusSucceeded ExecutionStatus = "succeeded"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
)

func (s ExecutionStatus) IsTerminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

func (s ExecutionStatus) String() string {
	return string(s)
}
