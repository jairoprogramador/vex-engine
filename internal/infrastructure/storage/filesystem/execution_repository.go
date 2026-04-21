package filesystem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/execution/aggregates"
	"github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
	"github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
)

// Debe transldarse al dominio executor

// ErrNotFound se retorna cuando una ejecución no existe en storage.
var ErrNotFound = errors.New("execution not found")

// executionDTO es la representación serializable de un Execution para JSON.
type executionDTO struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	ProjectID    string     `json:"project_id"`
	ProjectName  string     `json:"project_name"`
	PipelineURL  string     `json:"pipeline_url"`
	PipelineRef  string     `json:"pipeline_ref"`
	Step         string     `json:"step"`
	Environment  string     `json:"environment"`
	RuntimeImage string     `json:"runtime_image,omitempty"`
	RuntimeTag   string     `json:"runtime_tag,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	ExitCode     *int       `json:"exit_code,omitempty"`
}

// toDTO convierte un agregado Execution al DTO serializable.
func toDTO(e *aggregates.Execution) executionDTO {
	cfg := e.RuntimeConfig()
	return executionDTO{
		ID:           e.ID().String(),
		Status:       e.Status().String(),
		ProjectID:    e.ProjectID(),
		ProjectName:  e.ProjectName(),
		PipelineURL:  e.PipelineURL(),
		PipelineRef:  e.PipelineRef(),
		Step:         e.Step(),
		Environment:  e.Environment(),
		RuntimeImage: cfg.Image,
		RuntimeTag:   cfg.Tag,
		StartedAt:    e.StartedAt(),
		FinishedAt:   e.FinishedAt(),
		ExitCode:     e.ExitCode(),
	}
}

// fromDTO reconstruye un agregado Execution desde su DTO.
func fromDTO(dto executionDTO) (*aggregates.Execution, error) {
	id, err := vos.ExecutionIDFromString(dto.ID)
	if err != nil {
		return nil, fmt.Errorf("filesystem: rehydrate execution id: %w", err)
	}

	return aggregates.RehydrateExecution(
		id,
		vos.ExecutionStatus(dto.Status),
		dto.ProjectID,
		dto.ProjectName,
		dto.PipelineURL,
		dto.PipelineRef,
		dto.Step,
		dto.Environment,
		vos.NewRuntimeConfig(dto.RuntimeImage, dto.RuntimeTag),
		dto.StartedAt,
		dto.FinishedAt,
		dto.ExitCode,
	), nil
}

// ExecutionRepository persiste ejecuciones como archivos JSON en el filesystem.
// Cada ejecución se guarda en {basePath}/executions/{id}.json.
// Las escrituras son atómicas: se usa un archivo temporal y os.Rename para evitar
// lecturas de archivos parcialmente escritos.
type ExecutionRepository struct {
	basePath string
	mu       sync.RWMutex
}

// NewExecutionRepository crea un ExecutionRepository que persiste en basePath.
func NewExecutionRepository(basePath string) *ExecutionRepository {
	return &ExecutionRepository{basePath: basePath}
}

// filePath retorna la ruta del archivo JSON para una ejecución dada.
func (r *ExecutionRepository) filePath(id vos.ExecutionID) string {
	return filepath.Join(r.basePath, "executions", id.String()+".json")
}

// execDir retorna el directorio donde se almacenan las ejecuciones.
func (r *ExecutionRepository) execDir() string {
	return filepath.Join(r.basePath, "executions")
}

// writeAtomic serializa dto y lo escribe en dst usando una escritura atómica:
// escribe a dst+".tmp" y luego hace os.Rename al destino final.
func (r *ExecutionRepository) writeAtomic(dst string, dto executionDTO) error {
	data, err := json.MarshalIndent(dto, "", "\t")
	if err != nil {
		return fmt.Errorf("filesystem: marshal execution: %w", err)
	}

	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("filesystem: write temp file: %w", err)
	}

	if err := os.Rename(tmp, dst); err != nil {
		// Limpia el temporal si el rename falla para no dejar basura.
		_ = os.Remove(tmp)
		return fmt.Errorf("filesystem: rename temp file: %w", err)
	}

	return nil
}

// Save serializa el agregado a JSON y lo escribe en {basePath}/executions/{id}.json.
// Crea el directorio si no existe. La escritura es atómica.
func (r *ExecutionRepository) Save(_ context.Context, execution *aggregates.Execution) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(r.execDir(), 0o755); err != nil {
		return fmt.Errorf("filesystem: create executions dir: %w", err)
	}

	if err := r.writeAtomic(r.filePath(execution.ID()), toDTO(execution)); err != nil {
		return fmt.Errorf("filesystem: save execution %s: %w", execution.ID(), err)
	}

	return nil
}

// FindByID lee el archivo JSON para id y reconstruye el agregado.
// Retorna nil, nil si el archivo no existe.
func (r *ExecutionRepository) FindByID(_ context.Context, id vos.ExecutionID) (*aggregates.Execution, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := os.ReadFile(r.filePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("filesystem: read execution %s: %w", id, err)
	}

	var dto executionDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return nil, fmt.Errorf("filesystem: unmarshal execution %s: %w", id, err)
	}

	execution, err := fromDTO(dto)
	if err != nil {
		return nil, fmt.Errorf("filesystem: rehydrate execution %s: %w", id, err)
	}

	return execution, nil
}

// UpdateStatus actualiza el campo status de una ejecución existente.
// Si el archivo no existe retorna ErrNotFound. La escritura es atómica.
func (r *ExecutionRepository) UpdateStatus(_ context.Context, id vos.ExecutionID, status vos.ExecutionStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	path := r.filePath(id)

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("execution %s not found: %w", id, ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("filesystem: read execution %s: %w", id, err)
	}

	var dto executionDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		return fmt.Errorf("filesystem: unmarshal execution %s: %w", id, err)
	}

	dto.Status = status.String()

	if err := r.writeAtomic(path, dto); err != nil {
		return fmt.Errorf("filesystem: update status execution %s: %w", id, err)
	}

	return nil
}

// Verificación de contrato en compile-time.
var _ ports.ExecutionRepository = (*ExecutionRepository)(nil)
