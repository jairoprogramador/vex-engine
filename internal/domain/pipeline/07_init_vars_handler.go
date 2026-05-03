package pipeline

import (
	"context"
	"fmt"

	command "github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type InitVarsHandler struct {
	PipelineBaseHandler
}

var _ PipelineHandler = (*InitVarsHandler)(nil)

func NewInitVarsHandler() PipelineHandler {
	return &InitVarsHandler{
		PipelineBaseHandler: PipelineBaseHandler{Next: nil},
	}
}

func (h *InitVarsHandler) Handle(ctx *context.Context, request *PipelineRequestHandler) error {
	shortHash := request.ProjectHeadHash()
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}

	h.addInitVariable(request, command.VarProjectID, request.ProjectId(), false)
	h.addInitVariable(request, command.VarProjectName, request.ProjectName(), false)
	h.addInitVariable(request, command.VarProjectOrg, request.ProjectOrg(), false)
	h.addInitVariable(request, command.VarProjectTeam, request.ProjectTeam(), false)
	h.addInitVariable(request, command.VarProjectWorkdir, request.ProjectLocalPath(), false)

	h.addInitVariable(request, command.VarProjectVersion, request.ProjectVersion(), false)
	h.addInitVariable(request, command.VarProjectRevision, shortHash, false)
	h.addInitVariable(request, command.VarProjectRevisionFull, request.ProjectHeadHash(), false)
	h.addInitVariable(request, command.VarEnvironment, request.Environment(), false)
	h.addInitVariable(request, command.VarToolName, "vex", false)

	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}

func (h *InitVarsHandler) addInitVariable(request *PipelineRequestHandler, name, value string, isShared bool) {
	variable, err := command.NewVariable(name, value, isShared)
	if err != nil {
		request.Emit(fmt.Sprintf("error al crear variable de ejecución: %s", err.Error()))
	}
	request.AddAccumulatedVars(variable)
}
