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

// AddProducedVar anota que esta variable la PRODUJO el step en curso, y no sólo
// que ahora está en el mapa. Es la mitad que faltaba para poder distinguir lo que
// un step consume de lo que deja (spec 14 §6).
func (rh *CommandRequestHandler) AddProducedVar(variable Variable) {
	rh.executionContext.AddProducedVar(variable)
}

func (rh *CommandRequestHandler) AddCommandVar(commandVar CommandVariable) {
	rh.commandVars = append(rh.commandVars, commandVar)
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

func (rh *CommandRequestHandler) CommandShow() bool {
	return rh.command.Show()
}

func (rh *CommandRequestHandler) CommandWorkdir() string {
	return rh.command.Workdir().String()
}

func (rh *CommandRequestHandler) ProjectLocalPath() string {
	return rh.executionContext.ProjectLocalPath()
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

// CommandStatus deja de ser un campo que se asigna y nadie lee (BL-4): desde la
// spec 19 alimenta el `status` de `command_finished`.
func (rh *CommandRequestHandler) CommandStatus() CommandStatus {
	return rh.commandStatus
}

// CommandResult es donde `CommandResult` deja de morir dentro de su handler
// (BL-30). Se propaga hasta el emisor, que es quien necesita el exit code de un
// comando que terminó BIEN — el del que falló viaja dentro de su error.
func (rh *CommandRequestHandler) CommandResult() CommandResult {
	return rh.commandResult
}

func (rh *CommandRequestHandler) StepName() string {
	return rh.executionContext.Step().Name()
}
