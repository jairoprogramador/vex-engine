package utils

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

// RepositoryURL es la información mínima para derivar un directorio de clone estable y único.
type RepositoryURL interface {
	Name() string
	String() string
}

// LocalRepositoryPath devuelve la ruta absoluta o relativa baseDir/dirName donde dirName es
// "{último_segmento_de_Name}{8_primeros_hex_de_sha256(URL)}".
func LocalRepositoryPath(baseLocalPath string, repositoryURL RepositoryURL) string {
	dirName := DirNameFromUrl(repositoryURL)
	return filepath.Join(baseLocalPath, dirName)
}

func DirNameFromUrl(repositoryURL RepositoryURL) string {
	name := repositoryURL.Name()
	lastSegment := name
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		lastSegment = name[idx+1:]
	}
	lastSegment = strings.TrimSuffix(lastSegment, ".git")
	bytes := sha256.Sum256([]byte(repositoryURL.String()))
	suffix := fmt.Sprintf("%x", bytes)[:8]
	return fmt.Sprintf("%s%s", lastSegment, suffix)
}
