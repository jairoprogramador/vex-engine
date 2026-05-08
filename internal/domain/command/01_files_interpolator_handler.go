package command

import (
	"context"
	"fmt"
	"path/filepath"
)

type FilesInterpolatorHandler struct {
	CommandBaseHandler
	interpolator FileInterpolator
}

var _ CommandHandler = (*FilesInterpolatorHandler)(nil)

func NewFilesInterpolatorHandler(interpolator FileInterpolator) CommandHandler {
	return &FilesInterpolatorHandler{
		CommandBaseHandler: CommandBaseHandler{Next: nil},
		interpolator:       interpolator,
	}
}

func (h *FilesInterpolatorHandler) Handle(ctx *context.Context, request *CommandRequestHandler) error {
	localStepWorkdirPath, ok := request.LocalStepWorkdirPath()
	if !ok {
		return fmt.Errorf("variable de step workdir no encontrada")
	}
	workdir := filepath.Join(localStepWorkdirPath.Value(), request.CommandWorkdir())

	localTemplatesAbsPath := h.TemplateAbsPaths(workdir, request)
	session, err := h.interpolator.Interpolate(localTemplatesAbsPath, request.AccumulatedVars())
	if err != nil {
		return fmt.Errorf("aplicar templates: %w", err)
	}
	request.SetFileInterpolatorSession(session)
	if h.Next != nil {
		return h.Next.Handle(ctx, request)
	}
	return nil
}

func (h *FilesInterpolatorHandler) TemplateAbsPaths(localStepWorkdirPath string, request *CommandRequestHandler) []string {
	absPaths := make([]string, len(request.CommandTemplatePaths()))
	for i, templateRelativePath := range request.CommandTemplatePaths() {
		absPaths[i] = filepath.Join(localStepWorkdirPath, templateRelativePath.String())
	}
	return absPaths
}
