package status

import (
	"encoding/json"
	"fmt"
	"net/http"

	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

var _ domStepStatus.StatusRepository = (*SupabaseStatusRepository)(nil)

// SupabaseStatusRepository implementa StatusRepository invocando la edge function
// status-delete-step, que elimina el último registro de las 4 tablas de estado.
// La función resuelve internamente todos los IDs a partir del executionId y
// del nombre del step (parámetro idStep de la interfaz).
type SupabaseStatusRepository struct {
	endpoint    string
	token       string
	executionID string
	client      *http.Client
}

func NewSupabaseStatusRepository(
	endpoint, token, executionID string,
) domStepStatus.StatusRepository {
	return &SupabaseStatusRepository{
		endpoint:    endpoint,
		token:       token,
		executionID: executionID,
		client:      &http.Client{Timeout: supabaseStatusHTTPTimeout},
	}
}

// idStep es el nombre del step. Los demás parámetros se ignoran.
func (r *SupabaseStatusRepository) Delete(_, _, _, idStep string) error {
	payload, err := json.Marshal(map[string]any{
		"execution_id": r.executionID,
		"step_name":    idStep,
	})
	if err != nil {
		return fmt.Errorf("supabase status delete step: marshal: %w", err)
	}
	if _, err := supabasePost(r.client, r.endpoint, r.token, payload, supabaseStatusWriteRetries); err != nil {
		return fmt.Errorf("supabase status delete step: %w", err)
	}
	return nil
}
