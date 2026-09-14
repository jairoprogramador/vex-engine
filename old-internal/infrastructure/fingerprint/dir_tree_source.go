// Package fingerprint contiene las fuentes de árbol del puerto
// domain/fingerprint.TreeSource: el disco y la memoria. La regla de huella no
// vive aquí — vive en el dominio, y estas son las dos formas de alimentarla.
package fingerprint

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	domFingerprint "github.com/jairoprogramador/vex-engine/old-internal/domain/fingerprint"
)

var _ domFingerprint.TreeSource = (*DirTreeSource)(nil)

// DirTreeSource sirve un árbol que vive en el sistema de archivos.
type DirTreeSource struct {
	root string
}

// NewDirTreeSource resuelve la raíz y comprueba que sea un directorio.
//
// La raíz se resuelve con EvalSymlinks porque en modo local el proyecto ES un
// enlace al volumen montado: sin resolverlo, WalkDir no entraría —un enlace no
// es un directorio— y la huella saldría vacía sin un solo error.
func NewDirTreeSource(root string) (*DirTreeSource, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("dir tree source: acceder al directorio %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("dir tree source: %s no es un directorio", root)
	}

	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("dir tree source: resolver la raíz %s: %w", root, err)
	}

	return &DirTreeSource{root: resolved}, nil
}

// Walk recorre el árbol en el orden que exige el puerto: lexicográfico por
// nombre dentro de cada directorio, padre antes que contenido. Es el que da
// filepath.WalkDir.
func (s *DirTreeSource) Walk(fn domFingerprint.WalkFunc) error {
	return filepath.WalkDir(s.root, func(absPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// El árbol vivo cambia mientras se lee: lo que ya no está no entra.
			if os.IsNotExist(walkErr) {
				return nil
			}
			return fmt.Errorf("dir tree source: recorrer %s: %w", absPath, walkErr)
		}

		relPath, err := filepath.Rel(s.root, absPath)
		if err != nil {
			return fmt.Errorf("dir tree source: ruta relativa de %s: %w", absPath, err)
		}
		if relPath == "." {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("dir tree source: leer metadatos de %s: %w", absPath, err)
		}

		isSymlink := entry.Type()&fs.ModeSymlink != 0

		return fn(filepath.ToSlash(relPath), entry.IsDir(), isSymlink, info.Mode())
	})
}

// Open devuelve el contenido de un archivo regular o el destino de un enlace,
// sin seguirlo nunca.
func (s *DirTreeSource) Open(relPath string) (io.ReadCloser, error) {
	absPath := filepath.Join(s.root, filepath.FromSlash(relPath))

	info, err := os.Lstat(absPath)
	if err != nil {
		return nil, fmt.Errorf("dir tree source: leer metadatos de %s: %w", relPath, err)
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		target, err := os.Readlink(absPath)
		if err != nil {
			return nil, fmt.Errorf("dir tree source: leer el destino de %s: %w", relPath, err)
		}
		// Normalizado a "/": un enlace creado en Windows y otro en Unix hacia el
		// mismo sitio son el mismo contenido.
		return io.NopCloser(strings.NewReader(filepath.ToSlash(target))), nil
	}

	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("dir tree source: abrir %s: %w", relPath, err)
	}
	return file, nil
}
