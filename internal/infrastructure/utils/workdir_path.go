package utils

import (
	"path/filepath"
)

func EnvironmentWorkdirPath(workdirBasePath, environmentValue, stepName string, projectUrl, pipelineUrl RepositoryURL) string {
	projectDirName := DirNameFromUrl(projectUrl)
	pipelineDirName := DirNameFromUrl(pipelineUrl)
	return filepath.Join(workdirBasePath, projectDirName, pipelineDirName, environmentValue, stepName)
}

func SharedWorkdirPath(workdirBasePath, stepName string, projectUrl, pipelineUrl RepositoryURL) string {
	projectDirName := DirNameFromUrl(projectUrl)
	pipelineDirName := DirNameFromUrl(pipelineUrl)
	return filepath.Join(workdirBasePath, projectDirName, pipelineDirName, "shared", stepName)
}
