package step

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
)

var _ domStep.VarsStoreRepository = (*SupabaseVarsStoreRepository)(nil)

// ── Parámetros de transporte para la edge function store-vars ────────────────

const (
	// supabaseStoreHTTPTimeout limita el tiempo de cada POST a la edge function.
	supabaseStoreHTTPTimeout = 10 * time.Second

	// supabaseStoreGetRetries es el número máximo de intentos para Get.
	supabaseStoreGetRetries = 2

	// supabaseStoreWriteRetries es el número máximo de intentos para Save.
	supabaseStoreWriteRetries = 3
)

// supabaseStoreRetryBackoff es la pausa entre reintentos consecutivos.
var supabaseStoreRetryBackoff = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

// supabaseStoreVarDTO transporta un par nombre/valor. El flag isShared no viaja:
// lo reconstruyen los handlers 01/02 al cargar según el scope consultado.
type supabaseStoreVarDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SupabaseVarsStoreRepository implementa VarsStoreRepository vía edge function.
// La función resuelve internamente project_id y pipeline_id a partir del
// executionId; el scope y el nombre del step viajan en el payload. Por eso los
// parámetros projectUrl/pipelineUrl de la interfaz se ignoran (igual que en los
// repositorios Supabase de status).
type SupabaseVarsStoreRepository struct {
	endpoint    string
	token       string
	executionID string
	client      *http.Client
}

func NewSupabaseVarsStoreRepository(
	endpoint, token, executionID string,
) domStep.VarsStoreRepository {
	return &SupabaseVarsStoreRepository{
		endpoint:    endpoint,
		token:       token,
		executionID: executionID,
		client:      &http.Client{Timeout: supabaseStoreHTTPTimeout},
	}
}

func (r *SupabaseVarsStoreRepository) Get(
	_ *context.Context, _, _, scope, step string,
) ([]command.Variable, error) {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"scope":        scope,
		"step_name":    step,
		"operation":    "get",
	})
	if err != nil {
		return nil, fmt.Errorf("supabase vars store get: marshal: %w", err)
	}
	respBody, err := r.post(payload, supabaseStoreGetRetries)
	if err != nil {
		return nil, fmt.Errorf("supabase vars store get: %w", err)
	}
	var result struct {
		Variables []supabaseStoreVarDTO `json:"variables"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("supabase vars store get: decode response: %w", err)
	}

	isShared := scope == command.SharedScopeName
	variables := make([]command.Variable, 0, len(result.Variables))
	for _, dto := range result.Variables {
		variable, err := command.NewVariable(dto.Name, dto.Value, isShared)
		if err != nil {
			return nil, fmt.Errorf("supabase vars store get: crear variable %q: %w", dto.Name, err)
		}
		variables = append(variables, variable)
	}
	return variables, nil
}

func (r *SupabaseVarsStoreRepository) Save(
	_ *context.Context, _, _, scope, step string, vars []command.Variable,
) error {
	dtos := make([]supabaseStoreVarDTO, 0, len(vars))
	for _, variable := range vars {
		dtos = append(dtos, supabaseStoreVarDTO{Name: variable.Name(), Value: variable.Value()})
	}
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"scope":        scope,
		"step_name":    step,
		"operation":    "set",
		"variables":    dtos,
	})
	if err != nil {
		return fmt.Errorf("supabase vars store set: marshal: %w", err)
	}
	if _, err := r.post(payload, supabaseStoreWriteRetries); err != nil {
		return fmt.Errorf("supabase vars store set: %w", err)
	}
	return nil
}

// post envía el payload JSON al endpoint con hasta maxAttempts intentos y
// backoff entre ellos. Retorna el cuerpo de la respuesta exitosa (2xx).
func (r *SupabaseVarsStoreRepository) post(payload []byte, maxAttempts int) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(supabaseStoreRetryBackoff[attempt-1])
		}
		ctx, cancel := context.WithTimeout(context.Background(), supabaseStoreHTTPTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.endpoint, bytes.NewReader(payload))
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("build request: %w", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if r.token != "" {
			req.Header.Set("Authorization", "Bearer "+r.token)
		}
		resp, err := r.client.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("do request: %w", err)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}
		lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}
	return nil, lastErr
}
