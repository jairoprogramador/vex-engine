package vos

// ExecutionStatus representa el estado del ciclo de vida de una ejecución de pipeline.
type ExecutionStatus string

const (
	StatusQueued    ExecutionStatus = "queued"
	StatusRunning   ExecutionStatus = "running"
	StatusSucceeded ExecutionStatus = "succeeded"
	StatusFailed    ExecutionStatus = "failed"
	StatusCancelled ExecutionStatus = "cancelled"
)

// IsTerminal reporta si el estado es final (la ejecución no puede avanzar más).
func (s ExecutionStatus) IsTerminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// String devuelve la representación en string del ExecutionStatus.
func (s ExecutionStatus) String() string {
	return string(s)
}
