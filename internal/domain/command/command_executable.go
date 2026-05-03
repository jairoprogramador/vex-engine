package command

type CommandExecutable struct {
	BaseExecutable
	handler CommandHandler
}

var _ Executable = (*CommandExecutable)(nil)

func NewCommandExecutable(handler CommandHandler) *CommandExecutable {
	return &CommandExecutable{
		handler: handler,
	}
}

func (c *CommandExecutable) Execute(executionContext *ExecutionContext) error {
	return c.Run(
		executionContext,
		func() error {
			return nil
		},
		func() error {
			request := NewCommandRequestHandler(executionContext, executionContext.Command())
			err := c.handler.Handle(executionContext.Ctx(), request)
			if err == nil {
				request.MarkCommandSuccess()
			}
			return err
		},
		func() error {
			return nil
		},
	)
}
