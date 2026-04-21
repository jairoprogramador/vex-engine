package storage

import (
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

type PathResolver interface {
	Resolve(key vos.StorageKey) string
}

type DefaultPathResolver struct {
	rootVexPath string
}

func NewDefaultPathResolver(rootVexPath string) PathResolver {
	return &DefaultPathResolver{rootVexPath: rootVexPath}
}

func (r *DefaultPathResolver) Resolve(key vos.StorageKey) string {
	return filepath.Join(
		r.rootVexPath,
		key.ProjectName(),
		key.TemplateName(),
		"storage",
		key.Step().String()+".tb",
	)
}
