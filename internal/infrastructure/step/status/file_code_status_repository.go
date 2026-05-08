package status

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"errors"
	"io"
	"slices"

	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

var _ domStepStatus.CodeStatusRepository = (*FileCodeStatusRepository)(nil)

type FileCodeStatusRepository struct {
	statusBaseAbsolutePath string
}

func NewFileCodeStatusRepository(statusBaseAbsolutePath string) domStepStatus.CodeStatusRepository {
	return &FileCodeStatusRepository{statusBaseAbsolutePath: statusBaseAbsolutePath}
}

func (r *FileCodeStatusRepository) filePath(projectUrl string) string {
	projectName := utils.GetDirNameFromUrl(projectUrl)
	return filepath.Join(r.statusBaseAbsolutePath, projectName, statusDirName, codeStatusFileName)
}

func (r *FileCodeStatusRepository) getAll(projectUrl string) ([]FileCodeStatusDTO, error) {
	filePath := r.filePath(projectUrl)

	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []FileCodeStatusDTO{}, nil
		}
		return []FileCodeStatusDTO{}, fmt.Errorf("file code status repository: abrir archivo: %w", err)
	}
	defer file.Close()

	var fileCodeStatusArray []FileCodeStatusDTO
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&fileCodeStatusArray); err != nil {
		if errors.Is(err, io.EOF) {
			return []FileCodeStatusDTO{}, nil
		}
		return []FileCodeStatusDTO{}, fmt.Errorf("file code status repository: decodificación fallida: %v", err)
	}

	if len(fileCodeStatusArray) == 0 {
		return []FileCodeStatusDTO{}, nil
	}

	return fileCodeStatusArray, nil
}

func (r *FileCodeStatusRepository) Get(projectUrl string) (string, error) {
	fileCodeStatusArray, err := r.getAll(projectUrl)
	if err != nil {
		return "", err
	}

	if len(fileCodeStatusArray) == 0 {
		return "", nil
	}

	slices.SortFunc(fileCodeStatusArray, func(a, b FileCodeStatusDTO) int {
		return a.Date.Compare(b.Date)
	})

	lastFileCodeStatus := fileCodeStatusArray[len(fileCodeStatusArray)-1]
	return lastFileCodeStatus.Fingerprint, nil
}

func (r *FileCodeStatusRepository) Set(projectUrl string, fingerprint string) error {
	filePath := r.filePath(projectUrl)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("file code status repository: crear directorio %s: %w", dirPath, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file code status repository: crear archivo: %w", err)
	}
	defer file.Close()

	fileCodeStatusDto := ToFileCodeStatusDTO(fingerprint, time.Now())
	fileCodeStatusDtoArray := []FileCodeStatusDTO{fileCodeStatusDto}

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(fileCodeStatusDtoArray); err != nil {
		return fmt.Errorf("file code status repository: codificar file code status: %w", err)
	}

	return nil
}

func (r *FileCodeStatusRepository) Delete(projectUrl string) error {
	filePath := r.filePath(projectUrl)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file code status repository: eliminar archivo %s: %w", filePath, err)
	}
	return nil
}

func (r *FileCodeStatusRepository) add(projectUrl string, fingerprint string) error {
	fileCodeStatusArray, err := r.getAll(projectUrl)
	if err != nil {
		return err
	}
	fileCodeStatusArray = append(fileCodeStatusArray, ToFileCodeStatusDTO(fingerprint, time.Now()))

	filePath := r.filePath(projectUrl)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("file code status repository: crear directorio %s: %w", dirPath, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("file code status repository: crear archivo: %w", err)
	}
	defer file.Close()

	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(fileCodeStatusArray); err != nil {
		return fmt.Errorf("file code status repository: codificar file code status: %w", err)
	}

	return nil
}
