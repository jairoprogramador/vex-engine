package pipeline

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
)

var _ domPipeline.ProjectFingerprint = (*ProjectFingerprint)(nil)

type ProjectFingerprint struct{}

func NewProjectFingerprint() domPipeline.ProjectFingerprint {
	return &ProjectFingerprint{}
}

func (r *ProjectFingerprint) FromFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("project fingerprint: open file %s: %w", filePath, err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("project fingerprint: hash file %s: %w", filePath, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (r *ProjectFingerprint) FromDirectory(dirPath string) (string, error) {
	if _, err := os.Stat(dirPath); err != nil {
		return "", fmt.Errorf("project fingerprint: access directory %s: %w", dirPath, err)
	}

	ignoreDB, err := projectFingerprintBuildIgnoreDB(dirPath)
	if err != nil {
		return "", fmt.Errorf("project fingerprint: build ignore rules for %s: %w", dirPath, err)
	}

	var entries []string

	err = filepath.WalkDir(dirPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("walk error at %s: %w", path, walkErr)
		}

		relPath, err := filepath.Rel(dirPath, path)
		if err != nil {
			return fmt.Errorf("relative path for %s: %w", path, err)
		}

		if relPath == "." {
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}

		if !d.IsDir() && d.Name() == ".gitignore" {
			return nil
		}

		pathComponents := strings.Split(filepath.ToSlash(relPath), "/")

		if d.IsDir() {
			if ignoreDB.matches(pathComponents, true) {
				return filepath.SkipDir
			}
			return nil
		}

		if ignoreDB.matches(pathComponents, false) {
			return nil
		}

		fp, err := projectFingerprintHashFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("hash file %s: %w", path, err)
		}

		entries = append(entries, filepath.ToSlash(relPath)+":"+fp)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("project fingerprint: walk directory %s: %w", dirPath, err)
	}

	sort.Strings(entries)

	finalHasher := sha256.New()
	finalHasher.Write([]byte(strings.Join(entries, "\n")))

	return hex.EncodeToString(finalHasher.Sum(nil)), nil
}

func projectFingerprintHashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

type projectFingerprintIgnoreRule struct {
	pattern   string
	domain    []string
	dirOnly   bool
	inclusion bool
	anchored  bool
}

type projectFingerprintIgnoreDB struct {
	rules []projectFingerprintIgnoreRule
}

func projectFingerprintBuildIgnoreDB(root string) (*projectFingerprintIgnoreDB, error) {
	db := &projectFingerprintIgnoreDB{}

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			return walkErr
		}

		if !d.IsDir() {
			return nil
		}

		if d.Name() == ".git" {
			return filepath.SkipDir
		}

		gitignorePath := filepath.Join(path, ".gitignore")
		rules, err := projectFingerprintParseGitignoreFile(gitignorePath, root, path)
		if err != nil {
			return fmt.Errorf("parse gitignore at %s: %w", gitignorePath, err)
		}
		db.rules = append(db.rules, rules...)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return db, nil
}

func projectFingerprintParseGitignoreFile(path, root, dir string) ([]projectFingerprintIgnoreRule, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	relDir, err := filepath.Rel(root, dir)
	if err != nil {
		return nil, fmt.Errorf("rel path: %w", err)
	}

	var domain []string
	if relDir != "." {
		domain = strings.Split(filepath.ToSlash(relDir), "/")
	}

	var rules []projectFingerprintIgnoreRule
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimRight(line, " \t\r")

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rule := projectFingerprintIgnoreRule{domain: domain}

		if strings.HasPrefix(line, "!") {
			rule.inclusion = true
			line = line[1:]
		}

		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = line[:len(line)-1]
		}

		if strings.HasPrefix(line, "/") {
			rule.anchored = true
			line = line[1:]
		} else if strings.Contains(line, "/") {
			rule.anchored = true
		}

		rule.pattern = line
		rules = append(rules, rule)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	return rules, nil
}

func (db *projectFingerprintIgnoreDB) matches(pathComponents []string, isDir bool) bool {
	result := false

	for _, rule := range db.rules {
		if !projectFingerprintPathIsUnderDomain(pathComponents, rule.domain) {
			continue
		}

		relative := pathComponents[len(rule.domain):]

		if rule.dirOnly && !isDir {
			continue
		}

		if projectFingerprintMatchPattern(rule.pattern, relative, rule.anchored) {
			result = !rule.inclusion
		}
	}

	return result
}

func projectFingerprintPathIsUnderDomain(pathComponents, domain []string) bool {
	if len(pathComponents) <= len(domain) {
		return false
	}
	for i, d := range domain {
		if pathComponents[i] != d {
			return false
		}
	}
	return true
}

func projectFingerprintMatchPattern(pattern string, relative []string, anchored bool) bool {
	if len(relative) == 0 {
		return false
	}

	if strings.Contains(pattern, "**") {
		return projectFingerprintMatchDoubleGlob(pattern, relative)
	}

	if anchored {
		patParts := strings.Split(pattern, "/")
		return projectFingerprintMatchGlobSequence(patParts, relative)
	}

	for i := range relative {
		ok, err := filepath.Match(pattern, relative[i])
		if err == nil && ok {
			return true
		}
	}
	return false
}

func projectFingerprintMatchDoubleGlob(pattern string, relative []string) bool {
	patParts := strings.Split(pattern, "/")
	return projectFingerprintMatchDoubleGlobParts(patParts, relative)
}

func projectFingerprintMatchDoubleGlobParts(patParts, pathParts []string) bool {
	if len(patParts) == 0 {
		return len(pathParts) == 0
	}

	if patParts[0] == "**" {
		for i := 0; i <= len(pathParts); i++ {
			if projectFingerprintMatchDoubleGlobParts(patParts[1:], pathParts[i:]) {
				return true
			}
		}
		return false
	}

	if len(pathParts) == 0 {
		return false
	}

	ok, err := filepath.Match(patParts[0], pathParts[0])
	if err != nil || !ok {
		return false
	}

	return projectFingerprintMatchDoubleGlobParts(patParts[1:], pathParts[1:])
}

func projectFingerprintMatchGlobSequence(patParts, pathParts []string) bool {
	if len(patParts) > len(pathParts) {
		return false
	}
	for i, p := range patParts {
		ok, err := filepath.Match(p, pathParts[i])
		if err != nil || !ok {
			return false
		}
	}
	return true
}
