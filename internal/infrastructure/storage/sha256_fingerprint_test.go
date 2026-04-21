package storage

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

func hashFromString(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return hex.EncodeToString(hasher.Sum(nil))
}

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

func createTestDir(t *testing.T, files map[string]string) string {
	t.Helper()
	tmpDir := t.TempDir()
	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}
	return tmpDir
}

func TestSha256FingerprintService_FromFile(t *testing.T) {
	s := NewSha256Fingerprint()

	tests := []struct {
		name    string
		content string
	}{
		{name: "existing file", content: "hello world"},
		{name: "empty file", content: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectedHash := hashFromString(tt.content)
			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, "testfile.txt")
			require.NoError(t, os.WriteFile(filePath, []byte(tt.content), 0644))

			fp, err := s.FromFile(filePath)
			require.NoError(t, err)
			assert.Equal(t, expectedHash, fp.String())
		})
	}

	t.Run("nonexistent file returns no error", func(t *testing.T) {
		_, err := s.FromFile(filepath.Join(t.TempDir(), "nonexistent.txt"))
		assert.NoError(t, err)
	})
}

func TestSha256FingerprintService_FromDirectory(t *testing.T) {
	s := NewSha256Fingerprint()

	t.Run("simple directory", func(t *testing.T) {
		files := map[string]string{"file1.txt": "file1", "file2.txt": "file2"}
		expectedHash := calculateExpectedDirectoryHash(files)
		testDir := createTestDir(t, files)
		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("stable regardless of creation order", func(t *testing.T) {
		files := map[string]string{"file2.txt": "file2", "file1.txt": "file1"}
		expectedFiles := map[string]string{"file1.txt": "file1", "file2.txt": "file2"}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)
		testDir := createTestDir(t, files)
		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("respects root .gitignore", func(t *testing.T) {
		allFiles := map[string]string{
			".gitignore":             "logs/\n*.tmp\n/vendor",
			"file1.txt":              "file1",
			"file.tmp":               "temp file",
			"logs/log1.log":          "log content",
			"vendor/some-lib/lib.go": "lib content",
			"src/component.go":       "source",
		}
		expectedFiles := map[string]string{
			"file1.txt":        "file1",
			"src/component.go": "source",
		}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)
		testDir := createTestDir(t, allFiles)
		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run(".gitignore file does not contribute to hash", func(t *testing.T) {
		withGitignore := map[string]string{
			".gitignore": "*.log",
			"app.go":     "package main",
		}
		withoutGitignore := map[string]string{
			"app.go": "package main",
		}
		dirWith := createTestDir(t, withGitignore)
		dirWithout := createTestDir(t, withoutGitignore)

		fpWith, err := s.FromDirectory(dirWith)
		require.NoError(t, err)

		fpWithout, err := s.FromDirectory(dirWithout)
		require.NoError(t, err)

		assert.Equal(t, fpWithout.String(), fpWith.String(),
			"fingerprint must not change if only .gitignore differs")
	})

	t.Run("gitignore in subdirectory applies only to its scope", func(t *testing.T) {
		allFiles := map[string]string{
			"app.go":            "package main",
			"pkg/util.go":       "package pkg",
			"pkg/.gitignore":    "*.gen.go",
			"pkg/code.gen.go":   "// generated",
			"other/code.gen.go": "// also generated",
		}
		expectedFiles := map[string]string{
			"app.go":            "package main",
			"pkg/util.go":       "package pkg",
			"other/code.gen.go": "// also generated",
		}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)
		testDir := createTestDir(t, allFiles)
		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("gitignore in subdirectory does not appear in hash", func(t *testing.T) {
		withSubGitignore := map[string]string{
			"app.go":         "package main",
			"pkg/util.go":    "package pkg",
			"pkg/.gitignore": "*.gen.go",
		}
		withoutSubGitignore := map[string]string{
			"app.go":      "package main",
			"pkg/util.go": "package pkg",
		}
		dirWith := createTestDir(t, withSubGitignore)
		dirWithout := createTestDir(t, withoutSubGitignore)

		fpWith, err := s.FromDirectory(dirWith)
		require.NoError(t, err)

		fpWithout, err := s.FromDirectory(dirWithout)
		require.NoError(t, err)

		assert.Equal(t, fpWithout.String(), fpWith.String(),
			"fingerprint must not change if only a .gitignore in subdir differs")
	})

	t.Run("ignores .git directory", func(t *testing.T) {
		allFiles := map[string]string{
			"file1.txt":   "file1",
			".git/config": "git config",
			".git/HEAD":   "ref: refs/heads/main",
		}
		expectedFiles := map[string]string{"file1.txt": "file1"}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)
		testDir := createTestDir(t, allFiles)
		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("ignores symlinks", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "real.txt"), []byte("real content"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "target.txt"), []byte("target content"), 0644))
		require.NoError(t, os.Symlink(
			filepath.Join(tmpDir, "target.txt"),
			filepath.Join(tmpDir, "link.txt"),
		))

		expectedFiles := map[string]string{"real.txt": "real content", "target.txt": "target content"}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)

		fp, err := s.FromDirectory(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("same content produces same fingerprint", func(t *testing.T) {
		files := map[string]string{"a.go": "package a", "b.go": "package b"}
		dir1 := createTestDir(t, files)
		dir2 := createTestDir(t, files)

		fp1, err := s.FromDirectory(dir1)
		require.NoError(t, err)

		fp2, err := s.FromDirectory(dir2)
		require.NoError(t, err)

		assert.Equal(t, fp1.String(), fp2.String())
	})

	t.Run("content change alters fingerprint", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "app.go")
		require.NoError(t, os.WriteFile(filePath, []byte("version 1"), 0644))

		fp1, err := s.FromDirectory(tmpDir)
		require.NoError(t, err)

		require.NoError(t, os.WriteFile(filePath, []byte("version 2"), 0644))

		fp2, err := s.FromDirectory(tmpDir)
		require.NoError(t, err)

		assert.NotEqual(t, fp1.String(), fp2.String())
	})

	t.Run("paths use slash separator for cross-OS determinism", func(t *testing.T) {
		files := map[string]string{
			"subdir/nested/file.go": "package nested",
		}
		testDir := createTestDir(t, files)
		expectedFiles := map[string]string{
			"subdir/nested/file.go": "package nested",
		}
		expectedHash := calculateExpectedDirectoryHash(expectedFiles)

		fp, err := s.FromDirectory(testDir)
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("empty directory", func(t *testing.T) {
		expectedHash := calculateExpectedDirectoryHash(map[string]string{})
		fp, err := s.FromDirectory(t.TempDir())
		require.NoError(t, err)
		assert.Equal(t, expectedHash, fp.String())
	})

	t.Run("nonexistent directory returns error", func(t *testing.T) {
		_, err := s.FromDirectory("/path/to/non/existent/dir")
		assert.Error(t, err)
	})
}
