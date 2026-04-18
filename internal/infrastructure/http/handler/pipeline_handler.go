package handler

import (
	"net/http"

	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
)

// PipelineHandler gestiona los endpoints de operaciones sobre pipelines.
type PipelineHandler struct {
	validatePipeline *usecase.ValidatePipelineUseCase
}

// NewPipelineHandler construye el handler con el use case inyectado.
func NewPipelineHandler(validatePipeline *usecase.ValidatePipelineUseCase) *PipelineHandler {
	return &PipelineHandler{validatePipeline: validatePipeline}
}

// validateResponse es la proyección JSON del resultado de validación.
type validateResponse struct {
	Valid        bool     `json:"valid"`
	Steps        []string `json:"steps"`
	Environments []string `json:"environments"`
	Errors       []string `json:"errors"`
}

// Validate maneja GET /api/v1/pipelines/validate.
func (h *PipelineHandler) Validate(w http.ResponseWriter, r *http.Request) {
	pipelineURL := r.URL.Query().Get("pipeline_url")
	if pipelineURL == "" {
		writeError(w, http.StatusBadRequest, "pipeline_url is required")
		return
	}

	pipelineRef := r.URL.Query().Get("pipeline_ref")

	cmd := usecase.ValidatePipelineInput{
		PipelineUrl: pipelineURL,
		PipelineRef: pipelineRef,
	}

	result, err := h.validatePipeline.Execute(r.Context(), cmd)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, validateResponse{
		Valid:        result.Valid,
		Steps:        result.Steps,
		Environments: result.Environments,
		Errors:       result.Errors,
	})
}
