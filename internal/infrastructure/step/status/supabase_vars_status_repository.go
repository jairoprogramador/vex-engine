package status

import (
	"encoding/json"
	"fmt"
	"net/http"

	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

var _ domStepStatus.VariablesStatusRepository = (*SupabaseVarsStatusRepository)(nil)

// SupabaseVarsStatusRepository implementa VariablesStatusRepository vía edge function.
// La función resuelve internamente project_id, pipeline_id, environment_id y step_id
// a partir del executionId y del nombre del step (parámetro idStep de la interfaz).
type SupabaseVarsStatusRepository struct {
	endpoint    string
	token       string
	executionID string
	client      *http.Client
}

func NewSupabaseVarsStatusRepository(
	endpoint, token, executionID string,
) domStepStatus.VariablesStatusRepository {
	return &SupabaseVarsStatusRepository{
		endpoint:    endpoint,
		token:       token,
		executionID: executionID,
		client:      &http.Client{Timeout: supabaseStatusHTTPTimeout},
	}
}

// idStep es el nombre del step. Los demás parámetros se ignoran.
func (r *SupabaseVarsStatusRepository) Get(_, _, _, idStep string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "get",
	})
	if err != nil {
		return "", fmt.Errorf("supabase vars status get: marshal: %w", err)
	}
	respBody, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusGetRetries)
	if err != nil {
		return "", fmt.Errorf("supabase vars status get: %w", err)
	}
	var result struct {
		Fingerprint *string `json:"fingerprint"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("supabase vars status get: decode response: %w", err)
	}
	if result.Fingerprint == nil {
		return "", nil
	}
	return *result.Fingerprint, nil
}

func (r *SupabaseVarsStatusRepository) Set(_, _, _, idStep, fingerprint string) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "set",
		"fingerprint":  fingerprint,
	})
	if err != nil {
		return fmt.Errorf("supabase vars status set: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase vars status set: %w", err)
	}
	return nil
}

func (r *SupabaseVarsStatusRepository) Delete(_, _, _, idStep string) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "delete",
	})
	if err != nil {
		return fmt.Errorf("supabase vars status delete: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase vars status delete: %w", err)
	}
	return nil
}
