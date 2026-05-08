package execution

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
)

type FileSystemManager struct{}

func NewFileSystemManager() command.FileSystem {
	return &FileSystemManager{}
}

func (fs *FileSystemManager) ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error al leer el archivo %s: %w", path, err)
	}
	return data, nil
}

func (fs *FileSystemManager) WriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error al crear el directorio %s: %w", dir, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("error al escribir el archivo %s: %w", path, err)
	}
	return nil
}
