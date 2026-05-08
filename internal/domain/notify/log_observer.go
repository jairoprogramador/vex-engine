package notify

type LogObserver interface {
	Notify(executionID string, line string)
}
