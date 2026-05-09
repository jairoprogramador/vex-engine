package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

const (
	statusReporterDebounce = 200 * time.Millisecond
	statusReporterTimeout  = 5 * time.Second
)

// SupabaseStatusReporter implementa domNotify.StatusObserver para reportar
// transiciones intermedias de stage (best-effort, debounced) y expone
// ReportTerminal para reportar el cierre de la ejecución (best-effort con retry).
//
// La edge function `execution-status` se crea en M4; mientras no exista, los
// POST devolverán 404 — el reporter registra el fallo en stderr y continúa, sin
// abortar la ejecución (no es un fallo del pipeline). En modo local sin
// `--status-endpoint` se construye el reporter como nil/no se construye, y los
// callers usan MultiStatusObserver(nil) → noop.
type SupabaseStatusReporter struct {
	httpClient  *http.Client
	endpoint    string
	token       string
	executionID string

	mu              sync.Mutex
	lastNotifyAt    time.Time
	lastStageSent   string
	debounceWindow  time.Duration
	terminalBackoff []time.Duration
}

// NewSupabaseStatusReporter compone el reporter. executionID es el UUID que vexd
// recibió por flag/CLI; los POST lo incluyen en el body (no como path param).
func NewSupabaseStatusReporter(endpoint, token, executionID string) *SupabaseStatusReporter {
	return &SupabaseStatusReporter{
		httpClient:      &http.Client{Timeout: statusReporterTimeout},
		endpoint:        endpoint,
		token:           token,
		executionID:     executionID,
		debounceWindow:  statusReporterDebounce,
		terminalBackoff: []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second},
	}
}

// Notify reporta una transición de stage intermedia. Aplica debounce: si el
// último Notify fue hace <debounceWindow Y el mismo stage, se descarta.
// Best-effort con 1 retry; cualquier fallo se loguea a stderr.
func (r *SupabaseStatusReporter) Notify(executionID string, stage string) {
	r.mu.Lock()
	now := time.Now()
	if stage == r.lastStageSent && now.Sub(r.lastNotifyAt) < r.debounceWindow {
		r.mu.Unlock()
		return
	}
	r.lastNotifyAt = now
	r.lastStageSent = stage
	r.mu.Unlock()

	body := map[string]any{
		"execution_id":  r.executionID,
		"status":        "running",
		"current_stage": stage,
	}
	if err := r.postJSON(body, 1); err != nil {
		fmt.Fprintf(os.Stderr, "vexd: status reporter (stage=%s): %v\n", stage, err)
	}
}

// ReportTerminal envía el estado final de la ejecución al endpoint con retry
// 3× (500ms / 1s / 2s). Es la única señal que el portal/edge functions tienen
// de que el contenedor terminó: un fallo aquí deja la ejecución en "running"
// hasta que el TTL la marque como "error", por eso es retry-crítico.
func (r *SupabaseStatusReporter) ReportTerminal(status string, exitCode int, logsLost bool, errMsg string) error {
	body := map[string]any{
		"execution_id":  r.executionID,
		"status":        status,
		"exit_code":     exitCode,
		"logs_lost":     logsLost,
		"error_message": errMsg,
	}
	if err := r.postJSON(body, len(r.terminalBackoff)); err != nil {
		return fmt.Errorf("supabase status reporter terminal: %w", err)
	}
	return nil
}

// postJSON ejecuta el POST con N intentos (retries internos). El intento 0 va
// inmediatamente; los siguientes esperan el backoff[i-1].
func (r *SupabaseStatusReporter) postJSON(body map[string]any, attempts int) error {
	if r.endpoint == "" {
		return fmt.Errorf("status reporter endpoint not configured")
	}
	if attempts < 1 {
		attempts = 1
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 && i-1 < len(r.terminalBackoff) {
			time.Sleep(r.terminalBackoff[i-1])
		}

		ctx, cancel := context.WithTimeout(context.Background(), statusReporterTimeout)
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(payload))
		if reqErr != nil {
			cancel()
			lastErr = fmt.Errorf("build request: %w", reqErr)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if r.token != "" {
			req.Header.Set("Authorization", "Bearer "+r.token)
		}

		resp, doErr := r.httpClient.Do(req)
		if doErr != nil {
			cancel()
			lastErr = fmt.Errorf("post: %w", doErr)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		cancel()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("post returned status %d", resp.StatusCode)
	}
	return lastErr
}

var _ domNotify.StatusObserver = (*SupabaseStatusReporter)(nil)
