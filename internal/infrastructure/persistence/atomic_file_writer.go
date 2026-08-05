// Package persistence agrupa los colaboradores que dan durabilidad a los
// repositorios de archivo. No sabe qué se escribe —eso es del repositorio—,
// solo garantiza cómo se escribe.
package persistence

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// defaultFilePerm son los permisos del archivo publicado.
	defaultFilePerm os.FileMode = 0644

	// defaultDirPerm son los permisos de los directorios que haga falta crear.
	defaultDirPerm os.FileMode = 0755
)

// AtomicFileWriter publica el resultado de una serialización con rename, de
// forma que un lector nunca observa un archivo a medias: en `path` está el
// contenido anterior completo o el nuevo completo, nunca un estado intermedio.
//
// Es el único sitio del motor donde vive esa garantía. Los repositorios se
// limitan a construir su DTO y serializarlo (spec 02 §5.1).
type AtomicFileWriter struct {
	perm os.FileMode
}

// NewAtomicFileWriter construye un escritor con los permisos por defecto (0644).
func NewAtomicFileWriter() AtomicFileWriter {
	return AtomicFileWriter{perm: defaultFilePerm}
}

// NewAtomicFileWriterWithPerm construye un escritor con permisos explícitos.
func NewAtomicFileWriterWithPerm(perm os.FileMode) AtomicFileWriter {
	return AtomicFileWriter{perm: perm}
}

// Write serializa mediante encode sobre un archivo temporal en el MISMO
// directorio que path —rename entre filesystems distintos no es atómico— y lo
// publica con rename.
//
// Si encode, el Sync, el Close o el Chmod fallan, el temporal se elimina y el
// archivo previo queda intacto. Ningún error se descarta: un Write que retorna
// nil significa que el contenido nuevo está en disco.
func (w AtomicFileWriter) Write(path string, encode func(io.Writer) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, defaultDirPerm); err != nil {
		return fmt.Errorf("escritura atómica: crear directorio %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("escritura atómica: crear temporal en %s: %w", dir, err)
	}
	tmpPath := tmp.Name()

	if err := w.fill(tmp, encode); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("escritura atómica: %s: %w", path, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("escritura atómica: publicar %s: %w", path, err)
	}

	return nil
}

// fill escribe el contenido en el temporal y lo cierra. El Close no va en un
// defer desnudo: en el camino de escritura su error es el que revela que los
// datos no llegaron al archivo (spec 02 §5.4).
func (w AtomicFileWriter) fill(tmp *os.File, encode func(io.Writer) error) error {
	if err := encode(tmp); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("serializar: %w", err)
	}
	if err := tmp.Chmod(w.perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("permisos del temporal: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sincronizar temporal: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cerrar temporal: %w", err)
	}
	return nil
}
