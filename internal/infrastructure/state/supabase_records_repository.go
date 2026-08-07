package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domState "github.com/jairoprogramador/vex-engine/internal/domain/state"
)

var _ domState.Records = (*SupabaseRecordsRepository)(nil)

// ── Parámetros de transporte para la edge function store-vars ────────────────

const (
	// supabaseStoreHTTPTimeout limita el tiempo de cada POST a la edge function.
	supabaseStoreHTTPTimeout = 10 * time.Second

	// supabaseStoreGetRetries es el número máximo de intentos para leer.
	supabaseStoreGetRetries = 2

	// supabaseStoreWriteRetries es el número máximo de intentos para escribir.
	supabaseStoreWriteRetries = 3
)

// supabaseStoreRetryBackoff es la pausa entre reintentos consecutivos.
var supabaseStoreRetryBackoff = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

// supabaseStoreVarDTO transporta un par nombre/valor. El flag `shared` no viaja:
// lo reconstruye el ÁMBITO de la clave al leer.
type supabaseStoreVarDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SupabaseRecordsRepository pone la edge function `store-vars` detrás del puerto
// nuevo, **sin fingir que es append-only**.
//
// La edge function guarda el ÚLTIMO conjunto de variables por (scope, step) y no
// tiene historia, así que este adaptador:
//
//   - en `Append`, manda las variables del registro y descarta identificador,
//     huella y procedencia, que no tiene dónde guardar;
//   - en `Last`, devuelve un registro SIN ATRIBUIR (ver
//     `state.NewUnattributedRecord`), que por construcción no revive nunca.
//
// Consecuencia declarada: **en modo remoto ningún step revive**, exactamente
// como desde la spec 10 —donde el caché pasó a ser sólo de archivo y la máquina
// de Fly arranca fría en cada ejecución—. Lo que este adaptador conserva, y es
// justo lo que la spec 11 protege, es que **las variables con efecto real
// sobrevivan**: un ARN extraído en una ejecución remota sigue disponible en la
// siguiente.
//
// Se retira en la spec 16, con un destino de estado explícito.
type SupabaseRecordsRepository struct {
	endpoint    string
	token       string
	executionID string
	client      *http.Client
}

func NewSupabaseRecordsRepository(endpoint, token, executionID string) domState.Records {
	return &SupabaseRecordsRepository{
		endpoint:    endpoint,
		token:       token,
		executionID: executionID,
		client:      &http.Client{Timeout: supabaseStoreHTTPTimeout},
	}
}

func (r *SupabaseRecordsRepository) Last(
	_ *context.Context, key domState.Key,
) (domState.StepRecord, bool, error) {

	if key.IsZero() {
		return domState.StepRecord{}, false, fmt.Errorf("supabase records: clave vacía")
	}

	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"scope":        key.Scope().String(),
		"step_name":    key.StepID(),
		"operation":    "get",
	})
	if err != nil {
		return domState.StepRecord{}, false, fmt.Errorf("supabase records get: marshal: %w", err)
	}
	respBody, err := r.post(payload, supabaseStoreGetRetries)
	if err != nil {
		return domState.StepRecord{}, false, fmt.Errorf("supabase records get: %w", err)
	}
	var result struct {
		Variables []supabaseStoreVarDTO `json:"variables"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return domState.StepRecord{}, false, fmt.Errorf("supabase records get: decode response: %w", err)
	}
	if len(result.Variables) == 0 {
		return domState.StepRecord{}, false, nil
	}

	// El ámbito reconstruye la marca: el ámbito de proyecto es el que hasta la
	// spec 13 se llamaba `shared`.
	isShared := key.Scope().IsProject()
	variables := make([]command.Variable, 0, len(result.Variables))
	for _, dto := range result.Variables {
		variable, err := command.NewVariable(dto.Name, dto.Value, isShared, command.OriginState)
		if err != nil {
			return domState.StepRecord{}, false, fmt.Errorf(
				"supabase records get: crear variable %q: %w", dto.Name, err)
		}
		variables = append(variables, variable)
	}
	return domState.NewUnattributedRecord(variables), true, nil
}

func (r *SupabaseRecordsRepository) Append(
	_ *context.Context, key domState.Key, record domState.StepRecord,
) error {

	if key.IsZero() {
		return fmt.Errorf("supabase records: clave vacía")
	}

	variables := record.Variables()
	dtos := make([]supabaseStoreVarDTO, 0, len(variables))
	for i := range variables {
		dtos = append(dtos, supabaseStoreVarDTO{
			Name:  variables[i].Name(),
			Value: variables[i].Value(),
		})
	}
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"scope":        key.Scope().String(),
		"step_name":    key.StepID(),
		"operation":    "set",
		"variables":    dtos,
	})
	if err != nil {
		return fmt.Errorf("supabase records append: marshal: %w", err)
	}
	if _, err := r.post(payload, supabaseStoreWriteRetries); err != nil {
		return fmt.Errorf("supabase records append: %w", err)
	}
	return nil
}

// post envía el payload JSON al endpoint con hasta maxAttempts intentos y
// backoff entre ellos. Retorna el cuerpo de la respuesta exitosa (2xx).
func (r *SupabaseRecordsRepository) post(payload []byte, maxAttempts int) ([]byte, error) {
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
