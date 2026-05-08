package utils

import (
	"fmt"
	"path/filepath"
)

func VarsExecutionFilePath(storageBasePath, scopeName, stepName string, projectUrl, pipelineUrl RepositoryURL) string {
	projectDirName := DirNameFromUrl(projectUrl)
	pipelineDirName := DirNameFromUrl(pipelineUrl)
	return filepath.Join(storageBasePath, projectDirName, pipelineDirName, "variables", scopeName, fmt.Sprintf("%s.vars", stepName))
}

func StatusExecutionFilePath(storageBasePath, scopeName, stepName string, projectUrl, pipelineUrl RepositoryURL) string {
	projectDirName := DirNameFromUrl(projectUrl)
	pipelineDirName := DirNameFromUrl(pipelineUrl)
	return filepath.Join(storageBasePath, projectDirName, pipelineDirName, "status", scopeName, fmt.Sprintf("%s.status", stepName))
}
