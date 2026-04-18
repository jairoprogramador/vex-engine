package handler

import (
	"fmt"
	"net/http"

	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
)

// ExecutionLogsHandler gestiona GET /api/v1/executions/{id}/logs via SSE.
type ExecutionLogsHandler struct {
	streamLogs *usecase.LogsExecutionUseCase
	getExec    *usecase.GetExecutionUseCase
}

// NewExecutionLogsHandler construye el handler con los use cases inyectados.
func NewExecutionLogsHandler(
	streamLogs *usecase.LogsExecutionUseCase,
	getExec *usecase.GetExecutionUseCase,
) *ExecutionLogsHandler {
	return &ExecutionLogsHandler{
		streamLogs: streamLogs,
		getExec:    getExec,
	}
}

// ServeHTTP maneja el streaming SSE de logs de una ejecución.
func (h *ExecutionLogsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "streaming not supported")
		return
	}

	id := r.PathValue("id")

	logsChan, err := h.streamLogs.Execute(r.Context(), id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	for {
		select {
		case line, ok := <-logsChan:
			if !ok {
				finalStatus := h.resolveFinalStatus(r, id)
				fmt.Fprintf(w, "event: done\ndata: {\"status\":\"%s\"}\n\n", finalStatus) //nolint:errcheck
				flusher.Flush()
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", line) //nolint:errcheck
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *ExecutionLogsHandler) resolveFinalStatus(r *http.Request, id string) string {
	view, err := h.getExec.Execute(r.Context(), id)
	if err != nil {
		return "unknown"
	}
	return view.Status
}
