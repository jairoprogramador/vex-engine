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
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/persistence"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

var _ domStepStatus.CodeStatusRepository = (*FileCodeStatusRepository)(nil)

type FileCodeStatusRepository struct {
	statusBaseAbsolutePath string
	writer                 persistence.AtomicFileWriter
}

func NewFileCodeStatusRepository(statusBaseAbsolutePath string) domStepStatus.CodeStatusRepository {
	return &FileCodeStatusRepository{
		statusBaseAbsolutePath: statusBaseAbsolutePath,
		writer:                 persistence.NewAtomicFileWriter(),
	}
}

func (r *FileCodeStatusRepository) filePath(projectUrl, pipelineUrl, step string) string {
	projectName := utils.GetDirNameFromUrl(projectUrl)
	pipelineName := utils.GetDirNameFromUrl(pipelineUrl)
	codeFileName := fmt.Sprintf("code%s.status", step)
	return filepath.Join(r.statusBaseAbsolutePath, projectName, statusDirName, pipelineName, codeFileName)
}

func (r *FileCodeStatusRepository) getAll(projectUrl, pipelineUrl, step string) ([]FileCodeStatusDTO, error) {
	filePath := r.filePath(projectUrl, pipelineUrl, step)

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

func (r *FileCodeStatusRepository) Get(projectUrl, pipelineUrl, step string) (string, error) {
	fileCodeStatusArray, err := r.getAll(projectUrl, pipelineUrl, step)
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

func (r *FileCodeStatusRepository) Set(projectUrl, pipelineUrl, step, fingerprint string) error {
	filePath := r.filePath(projectUrl, pipelineUrl, step)
	fileCodeStatusDtoArray := []FileCodeStatusDTO{ToFileCodeStatusDTO(fingerprint, time.Now())}

	err := r.writer.Write(filePath, func(out io.Writer) error {
		return gob.NewEncoder(out).Encode(fileCodeStatusDtoArray)
	})
	if err != nil {
		return fmt.Errorf("file code status repository: escribir %s: %w", filePath, err)
	}

	return nil
}

func (r *FileCodeStatusRepository) Delete(projectUrl, pipelineUrl, step string) error {
	filePath := r.filePath(projectUrl, pipelineUrl, step)
	if err := os.Remove(filePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("file code status repository: eliminar archivo %s: %w", filePath, err)
	}
	return nil
}
