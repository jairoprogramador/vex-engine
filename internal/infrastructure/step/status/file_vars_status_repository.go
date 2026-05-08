package status

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

var _ domStepStatus.VariablesStatusRepository = (*FileVarsStatusRepository)(nil)

type FileVarsStatusRepository struct {
	statusBaseAbsolutePath string
}

func NewFileVarsStatusRepository(statusBaseAbsolutePath string) domStepStatus.VariablesStatusRepository {
	return &FileVarsStatusRepository{statusBaseAbsolutePath: statusBaseAbsolutePath}
}

func (r *FileVarsStatusRepository) filePath(projectUrl, pipelineUrl, environment, step string) string {
	projectName := utils.GetDirNameFromUrl(projectUrl)
	pipelineName := utils.GetDirNameFromUrl(pipelineUrl)
	return filepath.Join(r.statusBaseAbsolutePath, projectName, statusDirName, pipelineName, environment, step+".status")
}

func (r *FileVarsStatusRepository) getAll(projectUrl, pipelineUrl, environment, step string) ([]FileVarsStatusDTO, error) {
	filePath := r.filePath(projectUrl, pipelineUrl, environment, step)

	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []FileVarsStatusDTO{}, nil
		}
		return []FileVarsStatusDTO{}, fmt.Errorf("file vars status repository: abrir archivo: %w", err)
	}
	defer file.Close()

	var rows []FileVarsStatusDTO
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&rows); err != nil {
		if errors.Is(err, io.EOF) {
			return []FileVarsStatusDTO{}, nil
		}
		return []FileVarsStatusDTO{}, fmt.Errorf("file vars status repository: decodificación fallida: %v", err)
	}

	if len(rows) == 0 {
		return []FileVarsStatusDTO{}, nil
	}

	return rows, nil
}

func (r *FileVarsStatusRepository) Get(projectUrl, pipelineUrl, environment, step string) (string, error) {
	rows, err := r.getAll(projectUrl, pipelineUrl, environment, step)
	if err != nil {
		return "", err
	}

	if len(rows) == 0 {
		return "", nil
	}

	slices.SortFunc(rows, func(a, b FileVarsStatusDTO) int {
		return a.Date.Compare(b.Date)
	})

	last := rows[len(rows)-1]

	return last.Fingerprint, nil
}

func (r *FileVarsStatusRepository) Set(projectUrl, pipelineUrl, environment, step, fingerprint string) error {
	filePath := r.filePath(projectUrl, pipelineUrl, environment, step)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("file vars status repository: crear directorio %s: %w", dirPath, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file vars status repository: crear archivo: %w", err)
	}
	defer file.Close()

	row := ToFileVarsStatusDTO(fingerprint, time.Now())
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode([]FileVarsStatusDTO{row}); err != nil {
		return fmt.Errorf("file vars status repository: codificar vars status: %w", err)
	}

	return nil
}

func (r *FileVarsStatusRepository) Delete(projectUrl, pipelineUrl, environment, step string) error {
	filePath := r.filePath(projectUrl, pipelineUrl, environment, step)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file vars status repository: eliminar archivo %s: %w", filePath, err)
	}
	return nil
}

func (r *FileVarsStatusRepository) Add(projectUrl, pipelineUrl, environment, step string, fingerprint string) error {
	rows, err := r.getAll(projectUrl, pipelineUrl, environment, step)
	if err != nil {
		return err
	}
	rows = append(rows, ToFileVarsStatusDTO(fingerprint, time.Now()))

	filePath := r.filePath(projectUrl, pipelineUrl, environment, step)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("file vars status repository: crear directorio %s: %w", dirPath, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file vars status repository: crear archivo: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(rows); err != nil {
		return fmt.Errorf("file vars status repository: codificar vars status: %w", err)
	}

	return nil
}
