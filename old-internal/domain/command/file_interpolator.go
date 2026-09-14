package command

import (
	"fmt"
)

type FileSystem interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte) error
}

type FileInterpolator struct {
	fileSystem FileSystem
}

func NewFileInterpolator(fileSystem FileSystem) *FileInterpolator {
	return &FileInterpolator{fileSystem: fileSystem}
}

func (a *FileInterpolator) Interpolate(absoluteTemplatePaths []string, accumulatedVars *ExecutionVariableMap) (FileInterpolatorSession, error) {
	session := NewFileInterpolatorSession(a.fileSystem)

	for _, path := range absoluteTemplatePaths {
		original, err := a.fileSystem.ReadFile(path)
		if err != nil {
			_ = session.Restore()
			return FileInterpolatorSession{}, fmt.Errorf("leer template %s: %w", path, err)
		}
		session.backups[path] = original

		interpolated, err := Interpolate(string(original), accumulatedVars)
		if err != nil {
			_ = session.Restore()
			return FileInterpolatorSession{}, fmt.Errorf("interpolar template %s: %w", path, err)
		}

		if err := a.fileSystem.WriteFile(path, []byte(interpolated)); err != nil {
			_ = session.Restore()
			return FileInterpolatorSession{}, fmt.Errorf("escribir template interpolado %s: %w", path, err)
		}
	}

	return session, nil
}

type FileInterpolatorSession struct {
	backups    map[string][]byte
	fileSystem FileSystem
}

func NewFileInterpolatorSession(fileSystem FileSystem) FileInterpolatorSession {
	return FileInterpolatorSession{
		backups:    make(map[string][]byte),
		fileSystem: fileSystem,
	}
}

func (s *FileInterpolatorSession) Restore() error {
	var firstErr error
	for path, original := range s.backups {
		if err := s.fileSystem.WriteFile(path, original); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("restaurar template %s: %w", path, err)
		}
	}
	return firstErr
}
