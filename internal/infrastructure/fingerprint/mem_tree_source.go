package fingerprint

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	domFingerprint "github.com/jairoprogramador/vex-engine/internal/domain/fingerprint"
)

var _ domFingerprint.TreeSource = (*MemTreeSource)(nil)

// Permisos que MemTreeSource asigna a cada tipo de entrada. La regla v1 sólo
// mira el bit de ejecución, así que el resto es convención.
const (
	memFileMode       fs.FileMode = 0o644
	memExecutableMode fs.FileMode = 0o755
	memDirMode        fs.FileMode = fs.ModeDir | 0o755
	memSymlinkMode    fs.FileMode = fs.ModeSymlink | 0o777
)

// MemTreeSource sirve un árbol declarado en código, sin tocar disco.
//
// Es lo que hace verificable la especificación: los vectores de SPEC-v1.md se
// ejecutan sobre esta fuente, así que una implementación independiente puede
// reproducirlos sin replicar un sistema de archivos —ni pelearse con un
// .gitignore que git aplicaría al propio repositorio de tests—.
type MemTreeSource struct {
	root *memNode
}

type memNode struct {
	mode     fs.FileMode
	content  string // contenido del archivo, o destino del enlace
	children map[string]*memNode
}

func NewMemTreeSource() *MemTreeSource {
	return &MemTreeSource{root: newMemDir()}
}

func newMemDir() *memNode {
	return &memNode{mode: memDirMode, children: map[string]*memNode{}}
}

func (n *memNode) isDir() bool     { return n.mode.IsDir() }
func (n *memNode) isSymlink() bool { return n.mode&fs.ModeSymlink != 0 }

// AddDir declara un directorio, con sus padres implícitos.
func (s *MemTreeSource) AddDir(path string) *MemTreeSource {
	s.mkdirAll(splitPath(path))
	return s
}

// AddFile declara un archivo regular sin bit de ejecución.
func (s *MemTreeSource) AddFile(path, content string) *MemTreeSource {
	return s.add(path, &memNode{mode: memFileMode, content: content})
}

// AddExecutable declara un archivo regular con el bit de ejecución puesto.
func (s *MemTreeSource) AddExecutable(path, content string) *MemTreeSource {
	return s.add(path, &memNode{mode: memExecutableMode, content: content})
}

// AddSymlink declara un enlace simbólico hacia target. El destino no tiene que
// existir: no se sigue.
func (s *MemTreeSource) AddSymlink(path, target string) *MemTreeSource {
	return s.add(path, &memNode{mode: memSymlinkMode, content: target})
}

func (s *MemTreeSource) add(path string, node *memNode) *MemTreeSource {
	components := splitPath(path)
	if len(components) == 0 {
		return s
	}
	parent := s.mkdirAll(components[:len(components)-1])
	parent.children[components[len(components)-1]] = node
	return s
}

func (s *MemTreeSource) mkdirAll(components []string) *memNode {
	current := s.root
	for _, name := range components {
		child, ok := current.children[name]
		if !ok || !child.isDir() {
			child = newMemDir()
			current.children[name] = child
		}
		current = child
	}
	return current
}

// Walk recorre el árbol en orden lexicográfico dentro de cada directorio, padre
// antes que contenido, con la semántica de fs.SkipDir de filepath.WalkDir.
func (s *MemTreeSource) Walk(fn domFingerprint.WalkFunc) error {
	err := walkMemDir(s.root, "", fn)
	if errors.Is(err, fs.SkipAll) || errors.Is(err, fs.SkipDir) {
		return nil
	}
	return err
}

func walkMemDir(dir *memNode, dirPath string, fn domFingerprint.WalkFunc) error {
	names := make([]string, 0, len(dir.children))
	for name := range dir.children {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		child := dir.children[name]
		childPath := name
		if dirPath != "" {
			childPath = dirPath + "/" + name
		}

		err := fn(childPath, child.isDir(), child.isSymlink(), child.mode)
		if err != nil {
			if errors.Is(err, fs.SkipDir) {
				if child.isDir() {
					// El contenido del directorio se omite; sus hermanos no.
					continue
				}
				// Sobre una hoja, SkipDir omite el resto del directorio actual.
				return nil
			}
			return err
		}

		if child.isDir() {
			if err := walkMemDir(child, childPath, fn); err != nil {
				return err
			}
		}
	}

	return nil
}

// Open devuelve el contenido de un archivo regular o el destino de un enlace.
func (s *MemTreeSource) Open(path string) (io.ReadCloser, error) {
	node := s.lookup(splitPath(path))
	if node == nil {
		return nil, fmt.Errorf("mem tree source: %s: %w", path, fs.ErrNotExist)
	}
	if node.isDir() {
		return nil, fmt.Errorf("mem tree source: %s es un directorio", path)
	}
	return io.NopCloser(strings.NewReader(node.content)), nil
}

func (s *MemTreeSource) lookup(components []string) *memNode {
	current := s.root
	for _, name := range components {
		child, ok := current.children[name]
		if !ok {
			return nil
		}
		current = child
	}
	return current
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}
