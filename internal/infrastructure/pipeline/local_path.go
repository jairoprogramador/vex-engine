package pipeline

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
)

// resolveLocalPath deriva la ruta local de un repositorio a partir de su URL.
// Usa el último segmento del path más los primeros 8 caracteres del SHA-256 de
// la URL completa como sufijo para evitar colisiones entre repos con el mismo
// nombre en distintas organizaciones (e.g. org-a/myapp vs org-b/myapp).
func resolveLocalPath(baseDir string, url pipeline.RepositoryURL) string {
	name := url.Name()
	lastSegment := name
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		lastSegment = name[idx+1:]
	}
	h := sha256.Sum256([]byte(url.String()))
	suffix := fmt.Sprintf("%x", h)[:8]
	dirName := fmt.Sprintf("%s-%s", lastSegment, suffix)
	return filepath.Join(baseDir, dirName)
}
