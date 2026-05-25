package status

import (
	"encoding/json"
	"fmt"
	"net/http"

	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

var _ domStepStatus.InstructionsStatusRepository = (*SupabaseInstStatusRepository)(nil)

// SupabaseInstStatusRepository implementa InstructionsStatusRepository vía edge function.
// La función resuelve internamente project_id, pipeline_id y step_id a partir del
// executionId y del nombre del step (parámetro idStep de la interfaz).
type SupabaseInstStatusRepository struct {
	endpoint    string
	token       string
	executionID string
	client      *http.Client
}

func NewSupabaseInstStatusRepository(
	endpoint, token, executionID string,
) domStepStatus.InstructionsStatusRepository {
	return &SupabaseInstStatusRepository{
		endpoint:    endpoint,
		token:       token,
		executionID: executionID,
		client:      &http.Client{Timeout: supabaseStatusHTTPTimeout},
	}
}

// idStep es el nombre del step (e.g. "deploy") — la edge function lo usa para
// resolver step_id internamente. idProject e idPipeline se ignoran.
func (r *SupabaseInstStatusRepository) Get(_, _, idStep string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "get",
	})
	if err != nil {
		return "", fmt.Errorf("supabase inst status get: marshal: %w", err)
	}
	respBody, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusGetRetries)
	if err != nil {
		return "", fmt.Errorf("supabase inst status get: %w", err)
	}
	var result struct {
		Fingerprint *string `json:"fingerprint"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("supabase inst status get: decode response: %w", err)
	}
	if result.Fingerprint == nil {
		return "", nil
	}
	return *result.Fingerprint, nil
}

func (r *SupabaseInstStatusRepository) Set(_, _, idStep, fingerprint string) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "set",
		"fingerprint":  fingerprint,
	})
	if err != nil {
		return fmt.Errorf("supabase inst status set: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase inst status set: %w", err)
	}
	return nil
}

func (r *SupabaseInstStatusRepository) Delete(_, _, idStep string) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "delete",
	})
	if err != nil {
		return fmt.Errorf("supabase inst status delete: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase inst status delete: %w", err)
	}
	return nil
}
