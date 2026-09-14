package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"
)

const (
	// gitDirName es el único directorio que se salta por nombre, sin regla.
	gitDirName = ".git"
	// ignoreFileName es el archivo de reglas, que no participa en la huella.
	ignoreFileName = ".gitignore"

	// entrySeparator separa las entradas antes del hash final.
	entrySeparator = "\n"
	// fieldSeparator separa los campos dentro de una entrada.
	fieldSeparator = ":"
	// execMarker marca un archivo regular con el bit de ejecución puesto.
	execMarker = "x"
	// symlinkMarker marca un enlace simbólico.
	symlinkMarker = "l"
	// execBits son los únicos bits de permiso que entran en la huella.
	execBits fs.FileMode = 0o111
)

// Compute calcula la huella v1 del árbol servido por src.
//
// La regla completa y normativa está en SPEC-v1.md. En resumen: una entrada
// textual por archivo visible, ordenadas lexicográficamente, unidas por "\n" sin
// salto final, y sha256 del resultado.
func Compute(src TreeSource) (Fingerprint, error) {
	if src == nil {
		return Fingerprint{}, errors.New("fingerprint: fuente de árbol nula")
	}

	rules, err := buildIgnoreDB(src)
	if err != nil {
		return Fingerprint{}, fmt.Errorf("fingerprint: construir reglas de exclusión: %w", err)
	}

	var entries []string

	err = src.Walk(func(path string, isDir, isSymlink bool, mode fs.FileMode) error {
		name := baseName(path)

		if isDir && name == gitDirName {
			return fs.SkipDir
		}

		// El propio archivo de reglas no participa en la huella, sea regular o
		// enlace: cambiar un comentario en un .gitignore no cambia el código.
		if !isDir && name == ignoreFileName {
			return nil
		}

		components := strings.Split(path, "/")

		if isDir {
			if rules.matches(components, true) {
				return fs.SkipDir
			}
			return nil
		}

		if rules.matches(components, false) {
			return nil
		}

		entry, err := entryFor(src, path, isSymlink, mode)
		if err != nil {
			// Una entrada que desaparece a mitad del recorrido no es un error:
			// el árbol vivo cambia mientras se lee.
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return Fingerprint{}, fmt.Errorf("fingerprint: recorrer el árbol: %w", err)
	}

	sort.Strings(entries)

	hasher := sha256.New()
	hasher.Write([]byte(strings.Join(entries, entrySeparator)))

	return New(hex.EncodeToString(hasher.Sum(nil)))
}

// entryFor construye la entrada textual de una hoja del árbol:
//
//	archivo regular            → "<path>:<sha256(contenido)>"
//	archivo regular ejecutable → "<path>:<sha256(contenido)>:x"
//	enlace simbólico           → "<path>:<sha256(destino)>:l"
func entryFor(src TreeSource, path string, isSymlink bool, mode fs.FileMode) (string, error) {
	hash, err := hashOf(src, path)
	if err != nil {
		return "", err
	}

	entry := path + fieldSeparator + hash

	switch {
	case isSymlink:
		// El bit de ejecución de un enlace no significa nada: los permisos que
		// cuentan son los del destino, y el destino no se sigue.
		return entry + fieldSeparator + symlinkMarker, nil
	case mode&execBits != 0:
		return entry + fieldSeparator + execMarker, nil
	default:
		return entry, nil
	}
}

func hashOf(src TreeSource, path string) (string, error) {
	reader, err := src.Open(path)
	if err != nil {
		return "", fmt.Errorf("abrir %s: %w", path, err)
	}
	defer reader.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", fmt.Errorf("hashear %s: %w", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// buildIgnoreDB recorre el árbol una vez para acumular las reglas de todos los
// .gitignore, en el orden en que el recorrido los encuentra.
//
// Ese orden es la regla: gana la ÚLTIMA regla que casa, no la más profunda
// (divergencia (a), congelada en la v1). Las reglas se leen incluso dentro de
// directorios que otra regla ignora, igual que antes de la spec 08.
func buildIgnoreDB(src TreeSource) (*ignoreDB, error) {
	dirs := []string{""} // la raíz, que Walk no emite
	ignoreFiles := map[string]bool{}

	err := src.Walk(func(path string, isDir, isSymlink bool, _ fs.FileMode) error {
		name := baseName(path)

		if isDir {
			if name == gitDirName {
				return fs.SkipDir
			}
			dirs = append(dirs, path)
			return nil
		}

		// Sólo un archivo regular es un archivo de reglas: seguir un enlace
		// para leer reglas sacaría la identidad fuera del árbol.
		if !isSymlink && name == ignoreFileName {
			ignoreFiles[path] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	db := &ignoreDB{}
	for _, dir := range dirs {
		ignorePath := joinPath(dir, ignoreFileName)
		if !ignoreFiles[ignorePath] {
			continue
		}

		rules, err := parseIgnoreFile(src, ignorePath, dir)
		if err != nil {
			return nil, fmt.Errorf("leer %s: %w", ignorePath, err)
		}
		db.rules = append(db.rules, rules...)
	}

	return db, nil
}

func parseIgnoreFile(src TreeSource, ignorePath, dir string) ([]ignoreRule, error) {
	reader, err := src.Open(ignorePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	var domain []string
	if dir != "" {
		domain = strings.Split(dir, "/")
	}

	return parseIgnoreRules(string(content), domain), nil
}

// baseName es el último componente de una ruta con "/" como separador.
func baseName(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// joinPath une un directorio —"" es la raíz— con un nombre.
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}
