package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hashFromString calcula el hash SHA-256 de una cadena.
func hashFromString(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return hex.EncodeToString(hasher.Sum(nil))
}

// calculateExpectedDirectoryHash replica la lógica de hash del servicio para fines de prueba.
func calculateExpectedDirectoryHash(files map[string]string) string {
	if len(files) == 0 {
		return hashFromString("")
	}

	var fileHashes []string
	for path, content := range files {
		hash := hashFromString(content)
		fileHashes = append(fileHashes, fmt.Sprintf("%s:%s", path, hash))
	}

	sort.Strings(fileHashes)

	finalHasher := sha256.New()
	finalHasher.Write([]byte(strings.Join(fileHashes, "\n")))
	return hex.EncodeToString(finalHasher.Sum(nil))
}

func TestSha256FingerprintService_FromFile(t *testing.T) {
	s := NewSha256FingerprintService()

	t.Run("debería calcular el fingerprint de un archivo existente", func(t *testing.T) {
		const fileContent = "hello world"
		expectedHash := hashFromString(fileContent)

		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "testfile.txt")
		err := os.WriteFile(filePath, []byte(fileContent), 0644)
		require.NoError(t, err, "Failed to write test file")

		fp, err := s.FromFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("debería devolver un error si el archivo no existe", func(t *testing.T) {
		_, err := s.FromFile(filepath.Join(t.TempDir(), "nonexistent.txt"))
		assert.NoError(t, err)
	})

	t.Run("debería calcular el fingerprint de un archivo vacío", func(t *testing.T) {
		expectedHash := hashFromString("")

		emptyFilePath := filepath.Join(t.TempDir(), "empty.txt")
		err := os.WriteFile(emptyFilePath, []byte{}, 0644)
		require.NoError(t, err)

		fp, err := s.FromFile(emptyFilePath)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})
}

// createTestDir crea una estructura de directorios de prueba.
func createTestDir(t *testing.T, files map[string]string) string {
	t.Helper()
	tmpDir := t.TempDir()

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err, "Failed to create test directory structure")
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err, "Failed to write test file")
	}

	return tmpDir
}

func TestSha256FingerprintService_FromDirectory(t *testing.T) {
	s := NewSha256FingerprintService()

	t.Run("debería calcular el fingerprint de un directorio simple", func(t *testing.T) {
		files := map[string]string{
			"file1.txt": "file1",
			"file2.txt": "file2",
		}
		expectedHash := calculateExpectedDirectoryHash(files)
		testDir := createTestDir(t, files)

		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("el fingerprint debe ser estable independientemente del orden de creación de archivos", func(t *testing.T) {
		files := map[string]string{
			"file2.txt": "file2",
			"file1.txt": "file1",
		}
		// El mapa de archivos a hashear es el mismo, el helper se encarga de la consistencia.
		expectedFiles := map[string]string{
			"file1.txt": "file1",
			"file2.txt": "file2",
		}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)
		testDir := createTestDir(t, files)

		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String(), "El hash debe ser el mismo gracias a la ordenación")
	})

	t.Run("debería ignorar archivos y directorios según .gitignore", func(t *testing.T) {
		allFiles := map[string]string{
			".gitignore":             "logs/\n*.tmp\n/vendor",
			"file1.txt":              "file1",
			"file.tmp":               "temp file",
			"logs/log1.log":          "log content",
			"vendor/some-lib/lib.go": "lib content",
			"src/component.go":       "source",
		}
		// Solo estos archivos deben ser considerados para el hash
		expectedFiles := map[string]string{
			".gitignore":       "logs/\n*.tmp\n/vendor",
			"file1.txt":        "file1",
			"src/component.go": "source",
		}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)
		testDir := createTestDir(t, allFiles)

		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("debería ignorar el directorio .git", func(t *testing.T) {
		allFiles := map[string]string{
			"file1.txt":   "file1",
			".git/config": "git config",
			".git/HEAD":   "ref: refs/heads/main",
		}
		// Solo file1.txt debe ser considerado
		expectedFiles := map[string]string{
			"file1.txt": "file1",
		}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)
		testDir := createTestDir(t, allFiles)

		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("debería devolver el hash de un directorio vacío", func(t *testing.T) {
		expectedHash := calculateExpectedDirectoryHash(map[string]string{})
		tmpDir := t.TempDir()

		fp, err := s.FromDirectory(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("debería devolver un error para un directorio no existente", func(t *testing.T) {
		_, err := s.FromDirectory("/path/to/non/existent/dir")
		assert.Error(t, err)
	})
}
