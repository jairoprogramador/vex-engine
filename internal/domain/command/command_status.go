package command

type CommandStatus string

const (
	CommandSuccess CommandStatus = "SUCCESS"
	CommandFailure CommandStatus = "FAILURE"
)

func (s CommandStatus) String() string {
	return string(s)
}
