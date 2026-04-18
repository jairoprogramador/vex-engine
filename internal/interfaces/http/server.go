package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jairoprogramador/vex-engine/internal/application/usecase"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/http/handler"
	"github.com/jairoprogramador/vex-engine/internal/interfaces/http/middleware"
)

// Server encapsula el servidor HTTP de vexd.
type Server struct {
	httpServer *http.Server
}

// NewServer construye el servidor, registra todas las rutas y aplica el middleware de auth.
func NewServer(
	port string,
	authToken string,
	createExec *usecase.CreateExecutionUseCase,
	getExec *usecase.GetExecutionUseCase,
	streamLogs *usecase.LogsExecutionUseCase,
	deleteExec *usecase.DeleteExecutionUseCase,
	validatePipeline *usecase.ValidatePipelineUseCase,
) *Server {
	mux := http.NewServeMux()

	execHandler := handler.NewExecutionHandler(createExec, getExec, deleteExec)
	logsHandler := handler.NewExecutionLogsHandler(streamLogs, getExec)
	pipelineHandler := handler.NewPipelineHandler(validatePipeline)

	auth := func(h http.Handler) http.Handler {
		return middleware.Auth(authToken, h)
	}

	mux.Handle("POST /api/v1/executions", auth(http.HandlerFunc(execHandler.Create)))
	mux.Handle("GET /api/v1/executions/{id}", auth(http.HandlerFunc(execHandler.Get)))
	mux.Handle("DELETE /api/v1/executions/{id}", auth(http.HandlerFunc(execHandler.Delete)))
	mux.Handle("GET /api/v1/executions/{id}/logs", auth(logsHandler))
	mux.Handle("GET /api/v1/pipelines/validate", auth(http.HandlerFunc(pipelineHandler.Validate)))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	})

	return &Server{
		httpServer: &http.Server{
			Addr:    ":" + port,
			Handler: mux,
		},
	}
}

// ListenAndServe inicia el servidor HTTP. Bloquea hasta que falla o se llama Shutdown.
func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown detiene el servidor de forma controlada respetando el context dado.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
