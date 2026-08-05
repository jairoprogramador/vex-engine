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
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/persistence"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

var _ domStep.VarsStoreRepository = (*FileVarsStoreRepository)(nil)

type FileVarsStoreRepository struct {
	storageBaseAbsolutePath string
	writer                  persistence.AtomicFileWriter
}

func NewFileVarsStoreRepository(storageBaseAbsolutePath string) domStep.VarsStoreRepository {
	return &FileVarsStoreRepository{
		storageBaseAbsolutePath: storageBaseAbsolutePath,
		writer:                  persistence.NewAtomicFileWriter(),
	}
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

	// Un ámbito sin variables se escribe como una lista gob vacía, no como un
	// archivo de cero bytes. Por eso io.EOF aquí no es «no hay nada»: es un
	// archivo truncado, y el almacén no es desechable —de él sale estado real—,
	// así que se reporta en vez de degradarse a almacén vacío (spec 02 §1).
	var fileVarsStoreDto []FileVarStoreDTO
	decoder := gob.NewDecoder(file)
	if err := decoder.Decode(&fileVarsStoreDto); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("el archivo de variables '%s' está truncado o vacío", filePath)
		}
		return nil, fmt.Errorf("no se pudo decodificar el conjunto de variables desde '%s': %w", filePath, err)
	}

	variables, err := fromFileVarStoreDTO(fileVarsStoreDto)
	if err != nil {
		return []command.Variable{}, err
	}

	return variables, nil
}

// Save persiste el conjunto completo del ámbito. Una lista vacía escribe un
// archivo con cero entradas: es la única forma de expresar «este ámbito ya no
// tiene variables». Retornar nil dejaba las variables viejas en disco para
// siempre (spec 02 §5.3).
func (r *FileVarsStoreRepository) Save(_ *context.Context, projectUrl, pipelineUrl, scope, step string, variables []command.Variable) error {
	filePath := r.filePath(projectUrl, pipelineUrl, scope, step)
	fileVarsStoreDto := toFileVarStoreDTO(variables)

	err := r.writer.Write(filePath, func(out io.Writer) error {
		return gob.NewEncoder(out).Encode(fileVarsStoreDto)
	})
	if err != nil {
		return fmt.Errorf("no se pudo guardar el conjunto de variables en '%s': %w", filePath, err)
	}

	return nil
}
