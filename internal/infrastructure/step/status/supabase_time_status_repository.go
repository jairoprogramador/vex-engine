package status

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

var _ domStepStatus.TimeStatusRepository = (*SupabaseTimeStatusRepository)(nil)

// SupabaseTimeStatusRepository implementa TimeStatusRepository vía edge function.
// La función resuelve internamente project_id, environment_id y step_id a partir
// del executionId y del nombre del step (parámetro idStep de la interfaz).
// En Set, el parámetro time.Time se ignora: la DB registra now().
type SupabaseTimeStatusRepository struct {
	endpoint    string
	token       string
	executionID string
	client      *http.Client
}

func NewSupabaseTimeStatusRepository(
	endpoint, token, executionID string,
) domStepStatus.TimeStatusRepository {
	return &SupabaseTimeStatusRepository{
		endpoint:    endpoint,
		token:       token,
		executionID: executionID,
		client:      &http.Client{Timeout: supabaseStatusHTTPTimeout},
	}
}

// idStep es el nombre del step. idProject e idEnvironment se ignoran.
func (r *SupabaseTimeStatusRepository) Get(_, _, idStep string) (time.Time, error) {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "get",
	})
	if err != nil {
		return time.Time{}, fmt.Errorf("supabase time status get: marshal: %w", err)
	}
	respBody, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusGetRetries)
	if err != nil {
		return time.Time{}, fmt.Errorf("supabase time status get: %w", err)
	}
	var result struct {
		RecordedAt *string `json:"recorded_at"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return time.Time{}, fmt.Errorf("supabase time status get: decode response: %w", err)
	}
	if result.RecordedAt == nil {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, *result.RecordedAt)
	if err != nil {
		return time.Time{}, fmt.Errorf("supabase time status get: parse recorded_at %q: %w", *result.RecordedAt, err)
	}
	return t, nil
}

// El parámetro time.Time es ignorado; la DB registra now().
func (r *SupabaseTimeStatusRepository) Set(_, _, idStep string, _ time.Time) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "set",
	})
	if err != nil {
		return fmt.Errorf("supabase time status set: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase time status set: %w", err)
	}
	return nil
}

func (r *SupabaseTimeStatusRepository) Delete(_, _, idStep string) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "delete",
	})
	if err != nil {
		return fmt.Errorf("supabase time status delete: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase time status delete: %w", err)
	}
	return nil
}
