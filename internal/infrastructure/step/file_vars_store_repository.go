package step

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domStep "github.com/jairoprogramador/vex-engine/internal/domain/step"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

type FileVarsStoreRepository struct {
	storageBaseAbsolutePath string
}

func NewFileVarsStoreRepository(storageBaseAbsolutePath string) domStep.VarsStoreRepository {
	return &FileVarsStoreRepository{storageBaseAbsolutePath: storageBaseAbsolutePath}
}

func (r *FileVarsStoreRepository) filePath(projectUrl, pipelineUrl, scope, step string) string {
	projectName := utils.GetDirNameFromUrl(projectUrl)
	pipelineName := utils.GetDirNameFromUrl(pipelineUrl)
	return filepath.Join(r.storageBaseAbsolutePath, projectName, "store", pipelineName, scope, step+".vars")
}

func (r *FileVarsStoreRepository) Get(_ *context.Context, projectUrl, pipelineUrl, scope, step string) ([]command.Variable, error) {
	filePath := r.filePath(projectUrl, pipelineUrl, scope, step)

	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []command.Variable{}, nil
		}
		return nil, fmt.Errorf("no se pudo abrir el archivo de variables '%s': %w", filePath, err)
	}
	defer file.Close()

	var fileVarsStoreDto []FileVarStoreDTO
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&fileVarsStoreDto); err != nil {
		if errors.Is(err, io.EOF) {
			return []command.Variable{}, nil
		}
		return nil, fmt.Errorf("no se pudo decodificar el conjunto de variables desde '%s': %w", filePath, err)
	}

	variables, err := fromFileVarStoreDTO(fileVarsStoreDto)
	if err != nil {
		return []command.Variable{}, err
	}

	return variables, nil
}

func (r *FileVarsStoreRepository) Save(_ *context.Context, projectUrl, pipelineUrl, scope, step string, variables []command.Variable) error {
	if len(variables) == 0 {
		return nil
	}

	filePath := r.filePath(projectUrl, pipelineUrl, scope, step)

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("no se pudo crear el directorio para el archivo de conjunto de variables '%s': %w", dir, err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("no se pudo crear el archivo de conjunto de variables '%s': %w", filePath, err)
	}
	defer file.Close()

	fileVarsStoreDto := toFileVarStoreDTO(variables)
	encoder := gob.NewEncoder(file)
	if err := encoder.Encode(fileVarsStoreDto); err != nil {
		return fmt.Errorf("no se pudo codificar el conjunto de variables a '%s': %w", filePath, err)
	}

	return nil
}
