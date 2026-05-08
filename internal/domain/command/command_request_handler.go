package command

type CommandRequestHandler struct {
	executionContext       *ExecutionContext
	command                Command
	commandResult          CommandResult
	commandStatus          CommandStatus
	commandInterpolatedCmd string
	commandVars            []CommandVariable
}

func NewCommandRequestHandler(
	executionContext *ExecutionContext,
	command Command) *CommandRequestHandler {

	requestHandler := &CommandRequestHandler{
		executionContext: executionContext,
		command:          command,
		commandVars:      make([]CommandVariable, 0),
		commandResult:    NewCommandResult("", "", "", ""),
		commandStatus:    CommandFailure,
	}
	return requestHandler
}

func (rh *CommandRequestHandler) SetCommandInterpolatedCmd(commandInterpolatedCmd string) {
	rh.commandInterpolatedCmd = commandInterpolatedCmd
}

func (rh *CommandRequestHandler) SetFileInterpolatorSession(fileSession FileInterpolatorSession) {
	rh.executionContext.AddFileSession(fileSession)
}

func (rh *CommandRequestHandler) SetCommandResult(commandResult CommandResult) {
	rh.commandResult = commandResult
}

func (rh *CommandRequestHandler) AddAccumulatedVars(variable Variable) {
	rh.executionContext.AccumulatedVars().Add(variable)
}

func (rh *CommandRequestHandler) AddCommandVar(commandVar CommandVariable) {
	rh.commandVars = append(rh.commandVars, commandVar)
}

func (rh *CommandRequestHandler) CommandWorkdirIsShared() bool {
	return rh.command.Workdir().IsShared()
}

func (rh *CommandRequestHandler) CommandNormalizedStdout() string {
	return rh.commandResult.NormalizedStdout()
}

func (rh *CommandRequestHandler) CommandOutputs() []CommandOutput {
	return rh.command.Outputs()
}

func (rh *CommandRequestHandler) CommandInterpolatedCmd() string {
	return rh.commandInterpolatedCmd
}

func (rh *CommandRequestHandler) Emit(line string) {
	rh.executionContext.Emit(line)
}

func (rh *CommandRequestHandler) CommandName() string {
	return rh.command.Name()
}

func (rh *CommandRequestHandler) CommandWorkdir() string {
	return rh.command.Workdir().String()
}

func (rh *CommandRequestHandler) ExecutionID() ExecutionID {
	return rh.executionContext.ExecutionID()
}

func (rh *CommandRequestHandler) AccumulatedVars() *ExecutionVariableMap {
	return rh.executionContext.AccumulatedVars()
}

func (rh *CommandRequestHandler) LocalStepWorkdirPath() (Variable, bool) {
	return rh.executionContext.AccumulatedVars().Get(VarStepWorkdir)
}

func (rh *CommandRequestHandler) CommandCmd() string {
	return rh.command.Cmd()
}

func (rh *CommandRequestHandler) CommandTemplatePaths() []CommandTemplatePath {
	return rh.command.TemplatePaths()
}

func (rh *CommandRequestHandler) MarkCommandSuccess() {
	rh.commandStatus = CommandSuccess
}

func (rh *CommandRequestHandler) StepName() string {
	return rh.executionContext.Step().Name()
}
