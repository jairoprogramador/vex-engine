package utils

import (
	"fmt"
	"path/filepath"
)

func StepPipelinePath(pipelinesBasePath, stepNameDir string) string {
	return filepath.Join(pipelinesBasePath, "steps", stepNameDir)
}

func VarsPipelineFilePath(pipelinesBasePath, environmentValue, stepName string) string {
	return filepath.Join(pipelinesBasePath, "variables", environmentValue, fmt.Sprintf("%s.yaml", stepName))
}
