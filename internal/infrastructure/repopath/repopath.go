package repopath

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
)

// CloneURL es la información mínima para derivar un directorio de clone estable y único.
type CloneURL interface {
	Name() string
	String() string
}

// ForClone devuelve la ruta absoluta o relativa baseDir/dirName donde dirName es
// "{último_segmento_de_Name}-{8_primeros_hex_de_sha256(URL)}".
func ForClone(baseDir string, u CloneURL) string {
	name := u.Name()
	lastSegment := name
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		lastSegment = name[idx+1:]
	}
	lastSegment = strings.TrimSuffix(lastSegment, ".git")
	h := sha256.Sum256([]byte(u.String()))
	suffix := fmt.Sprintf("%x", h)[:8]
	dirName := fmt.Sprintf("%s-%s", lastSegment, suffix)
	return filepath.Join(baseDir, dirName)
}
