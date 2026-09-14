package command

type Executable interface {
	Execute(ctx *ExecutionContext) error
}
