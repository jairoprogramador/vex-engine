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

var _ domStepStatus.TimeStatusRepository = (*FileTimeStatusRepository)(nil)

type FileTimeStatusRepository struct {
	statusBaseAbsolutePath string
}

func NewFileTimeStatusRepository(statusBaseAbsolutePath string) domStepStatus.TimeStatusRepository {
	return &FileTimeStatusRepository{statusBaseAbsolutePath: statusBaseAbsolutePath}
}

func (r *FileTimeStatusRepository) filePath(projectUrl, environment, step string) string {
	projectName := utils.GetDirNameFromUrl(projectUrl)
	timeStatusFileName := fmt.Sprintf("time%s.status", step)
	return filepath.Join(r.statusBaseAbsolutePath, projectName, statusDirName, environment, timeStatusFileName)
}

func (r *FileTimeStatusRepository) getAll(projectUrl, environment, step string) ([]FileTimeStatusDTO, error) {
	filePath := r.filePath(projectUrl, environment, step)

	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []FileTimeStatusDTO{}, nil
		}
		return []FileTimeStatusDTO{}, fmt.Errorf("file time status repository: abrir archivo: %w", err)
	}
	defer file.Close()

	var rows []FileTimeStatusDTO
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&rows); err != nil {
		if errors.Is(err, io.EOF) {
			return []FileTimeStatusDTO{}, nil
		}
		return []FileTimeStatusDTO{}, fmt.Errorf("file time status repository: decodificación fallida: %v", err)
	}

	if len(rows) == 0 {
		return []FileTimeStatusDTO{}, nil
	}

	return rows, nil
}

func (r *FileTimeStatusRepository) Get(projectUrl, environment, step string) (time.Time, error) {
	rows, err := r.getAll(projectUrl, environment, step)
	if err != nil {
		return time.Time{}, err
	}

	if len(rows) == 0 {
		return time.Time{}, nil
	}

	slices.SortFunc(rows, func(a, b FileTimeStatusDTO) int {
		return a.Date.Compare(b.Date)
	})

	last := rows[len(rows)-1]

	return last.At, nil
}

func (r *FileTimeStatusRepository) Set(projectUrl, environment, step string, at time.Time) error {
	filePath := r.filePath(projectUrl, environment, step)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("file time status repository: crear directorio %s: %w", dirPath, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file time status repository: crear archivo: %w", err)
	}
	defer file.Close()

	row := ToFileTimeStatusDTO(at, time.Now())
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode([]FileTimeStatusDTO{row}); err != nil {
		return fmt.Errorf("file time status repository: codificar time status: %w", err)
	}

	return nil
}

func (r *FileTimeStatusRepository) Delete(projectUrl string, environment, step string) error {
	filePath := r.filePath(projectUrl, environment, step)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file time status repository: eliminar archivo %s: %w", filePath, err)
	}
	return nil
}

func (r *FileTimeStatusRepository) add(projectUrl, environment, step string, at time.Time) error {
	rows, err := r.getAll(projectUrl, environment, step)
	if err != nil {
		return err
	}
	rows = append(rows, ToFileTimeStatusDTO(at, time.Now()))

	filePath := r.filePath(projectUrl, environment, step)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("file time status repository: crear directorio %s: %w", dirPath, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file time status repository: crear archivo: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(rows); err != nil {
		return fmt.Errorf("file time status repository: codificar time status: %w", err)
	}

	return nil
}
