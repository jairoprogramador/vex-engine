package vos

import (
	"fmt"
	"strings"
)

const (
	varsExtension = "var"
)

type FileName struct {
	value string
}

func NewFileName(name, extension string) (FileName, error) {
	if name == "" {
		return FileName{}, fmt.Errorf("base name for file cannot be empty")
	}
	if extension == "" {
		return FileName{}, fmt.Errorf("extension for file cannot be empty")
	}
	cleanExtension := strings.TrimPrefix(extension, ".")
	return FileName{value: fmt.Sprintf("%s.%s", name, cleanExtension)}, nil
}

func NewVarsFileName(stepName string) (FileName, error) {
	return NewFileName(stepName, varsExtension)
}

func (f FileName) String() string {
	return f.value
}
