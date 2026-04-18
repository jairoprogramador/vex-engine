package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jairoprogramador/vex-engine/internal/application/dto"
	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
)

// ExecutionHandler gestiona los endpoints CRUD de ejecuciones.
type ExecutionHandler struct {
	createExec *usecase.CreateExecutionUseCase
	getExec    *usecase.GetExecutionUseCase
	deleteExec *usecase.DeleteExecutionUseCase
}

// NewExecutionHandler construye el handler con los use cases inyectados.
func NewExecutionHandler(
	createExec *usecase.CreateExecutionUseCase,
	getExec *usecase.GetExecutionUseCase,
	deleteExec *usecase.DeleteExecutionUseCase,
) *ExecutionHandler {
	return &ExecutionHandler{
		createExec: createExec,
		getExec:    getExec,
		deleteExec: deleteExec,
	}
}

// createRequest es el body JSON esperado en POST /api/v1/executions.
type createRequest struct {
	Project   projectInput   `json:"project"`
	Pipeline  pipelineInput  `json:"pipeline"`
	Execution executionInput `json:"execution"`
}

type projectInput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Team string `json:"team"`
	Org  string `json:"org"`
	URL  string `json:"url"`
	Ref  string `json:"ref"`
}

type pipelineInput struct {
	URL string `json:"url"`
	Ref string `json:"ref"`
}

type executionInput struct {
	Step         string `json:"step"`
	Environment  string `json:"environment"`
	RuntimeImage string `json:"runtime_image"`
	RuntimeTag   string `json:"runtime_tag"`
}

// Create maneja POST /api/v1/executions.
func (h *ExecutionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	cmd := dto.RequestInput{
		Project: dto.ProjectInput{
			ID:   req.Project.ID,
			Name: req.Project.Name,
			Team: req.Project.Team,
			Org:  req.Project.Org,
			URL:  req.Project.URL,
			Ref:  req.Project.Ref,
		},
		Pipeline: dto.PipelineInput{
			URL: req.Pipeline.URL,
			Ref: req.Pipeline.Ref,
		},
		Execution: dto.ExecutionInput{
			Step:         req.Execution.Step,
			Environment:  req.Execution.Environment,
			RuntimeImage: req.Execution.RuntimeImage,
			RuntimeTag:   req.Execution.RuntimeTag,
		},
	}

	result, err := h.createExec.Execute(r.Context(), cmd)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"execution_id": result.ExecutionID,
		"status":       result.Status,
	})
}

// executionResponse es la proyección JSON de una ejecución.
type executionResponse struct {
	ExecutionID string  `json:"execution_id"`
	Status      string  `json:"status"`
	Step        string  `json:"step"`
	Environment string  `json:"environment"`
	ProjectID   string  `json:"project_id"`
	ProjectName string  `json:"project_name"`
	StartedAt   string  `json:"started_at"`
	FinishedAt  *string `json:"finished_at"`
	ExitCode    *int    `json:"exit_code"`
}

// Get maneja GET /api/v1/executions/{id}.
func (h *ExecutionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	view, err := h.getExec.Execute(r.Context(), id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := executionResponse{
		ExecutionID: view.ExecutionID,
		Status:      view.Status,
		Step:        view.Step,
		Environment: view.Environment,
		ProjectID:   view.ProjectID,
		ProjectName: view.ProjectName,
		StartedAt:   view.StartedAt.String(),
		ExitCode:    view.ExitCode,
	}
	if view.FinishedAt != nil {
		s := view.FinishedAt.String()
		resp.FinishedAt = &s
	}

	writeJSON(w, http.StatusOK, resp)
}

// Delete maneja DELETE /api/v1/executions/{id}.
func (h *ExecutionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	result, err := h.deleteExec.Execute(r.Context(), id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"execution_id": result.ExecutionID,
		"status":       result.Status,
	})
}
