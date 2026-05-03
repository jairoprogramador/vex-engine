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

var _ domStepStatus.InstructionsStatusRepository = (*FileInstStatusRepository)(nil)

type FileInstStatusRepository struct {
	statusBaseAbsolutePath string
}

func NewFileInstStatusRepository(statusBaseAbsolutePath string) domStepStatus.InstructionsStatusRepository {
	return &FileInstStatusRepository{statusBaseAbsolutePath: statusBaseAbsolutePath}
}

func (r *FileInstStatusRepository) filePath(projectUrl, pipelineUrl, step string) string {
	projectName := utils.GetDirNameFromUrl(projectUrl)
	pipelineName := utils.GetDirNameFromUrl(pipelineUrl)
	instFileName := fmt.Sprintf("inst%s.status", step)
	return filepath.Join(r.statusBaseAbsolutePath, projectName, statusDirName, pipelineName, instFileName)
}

func (r *FileInstStatusRepository) getAll(projectUrl, pipelineUrl, step string) ([]FileInstStatusDTO, error) {
	filePath := r.filePath(projectUrl, pipelineUrl, step)

	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []FileInstStatusDTO{}, nil
		}
		return []FileInstStatusDTO{}, fmt.Errorf("file inst status repository: abrir archivo: %w", err)
	}
	defer file.Close()

	var fileInstStatusArray []FileInstStatusDTO
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&fileInstStatusArray); err != nil {
		if errors.Is(err, io.EOF) {
			return []FileInstStatusDTO{}, nil
		}
		return []FileInstStatusDTO{}, fmt.Errorf("file inst status repository: decodificación fallida: %v", err)
	}

	if len(fileInstStatusArray) == 0 {
		return []FileInstStatusDTO{}, nil
	}

	return fileInstStatusArray, nil
}

func (r *FileInstStatusRepository) Get(projectUrl, pipelineUrl, step string) (string, error) {
	fileInstStatusArray, err := r.getAll(projectUrl, pipelineUrl, step)
	if err != nil {
		return "", err
	}

	if len(fileInstStatusArray) == 0 {
		return "", nil
	}

	slices.SortFunc(fileInstStatusArray, func(a, b FileInstStatusDTO) int {
		return a.Date.Compare(b.Date)
	})

	last := fileInstStatusArray[len(fileInstStatusArray)-1]

	return last.Fingerprint, nil
}

func (r *FileInstStatusRepository) Set(projectUrl, pipelineUrl, step, fingerprint string) error {
	filePath := r.filePath(projectUrl, pipelineUrl, step)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("file inst status repository: crear directorio %s: %w", dirPath, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file inst status repository: crear archivo: %w", err)
	}
	defer file.Close()

	row := ToFileInstStatusDTO(fingerprint, time.Now())
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode([]FileInstStatusDTO{row}); err != nil {
		return fmt.Errorf("file inst status repository: codificar inst status: %w", err)
	}

	return nil
}

func (r *FileInstStatusRepository) Delete(projectUrl, pipelineUrl, step string) error {
	filePath := r.filePath(projectUrl, pipelineUrl, step)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file inst status repository: eliminar archivo %s: %w", filePath, err)
	}
	return nil
}

func (r *FileInstStatusRepository) add(projectUrl, pipelineUrl, step, fingerprint string) error {
	fileInstStatusArray, err := r.getAll(projectUrl, pipelineUrl, step)
	if err != nil {
		return err
	}
	fileInstStatusArray = append(fileInstStatusArray, ToFileInstStatusDTO(fingerprint, time.Now()))

	filePath := r.filePath(projectUrl, pipelineUrl, step)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("file inst status repository: crear directorio %s: %w", dirPath, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file inst status repository: crear archivo: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(fileInstStatusArray); err != nil {
		return fmt.Errorf("file inst status repository: codificar inst status: %w", err)
	}

	return nil
}
