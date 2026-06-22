package status

import (
	"encoding/json"
	"fmt"
	"net/http"

	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

var _ domStepStatus.CodeStatusRepository = (*SupabaseCodeStatusRepository)(nil)

// SupabaseCodeStatusRepository implementa CodeStatusRepository persistiendo
// el fingerprint de código de proyecto en step_status_code vía edge function.
// Se usa en modo remoto (Fly Machine); el modo local usa FileCodeStatusRepository.
//
// La edge function resuelve internamente project_id, pipeline_id y step_id a
// partir del executionId y del nombre del step (parámetro idStep de la interfaz).
// Los parámetros idProject e idPipeline se ignoran.
type SupabaseCodeStatusRepository struct {
	endpoint    string
	token       string
	executionID string
	client      *http.Client
}

func NewSupabaseCodeStatusRepository(
	endpoint, token, executionID string,
) domStepStatus.CodeStatusRepository {
	return &SupabaseCodeStatusRepository{
		endpoint:    endpoint,
		token:       token,
		executionID: executionID,
		client:      &http.Client{Timeout: supabaseStatusHTTPTimeout},
	}
}

func (r *SupabaseCodeStatusRepository) Get(_, _, idStep string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "get",
	})
	if err != nil {
		return "", fmt.Errorf("supabase code status get: marshal: %w", err)
	}
	respBody, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusGetRetries)
	if err != nil {
		return "", fmt.Errorf("supabase code status get: %w", err)
	}
	var result struct {
		Fingerprint *string `json:"fingerprint"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("supabase code status get: decode response: %w", err)
	}
	if result.Fingerprint == nil {
		return "", nil
	}
	return *result.Fingerprint, nil
}

func (r *SupabaseCodeStatusRepository) Set(_, _, idStep, fingerprint string) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "set",
		"fingerprint":  fingerprint,
	})
	if err != nil {
		return fmt.Errorf("supabase code status set: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase code status set: %w", err)
	}
	return nil
}

func (r *SupabaseCodeStatusRepository) Delete(_, _, idStep string) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
		"operation":    "delete",
	})
	if err != nil {
		return fmt.Errorf("supabase code status delete: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase code status delete: %w", err)
	}
	return nil
}
