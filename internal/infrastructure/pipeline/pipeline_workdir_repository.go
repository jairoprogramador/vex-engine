package pipeline

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	domPipeline "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
	"github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

const gitDirName = ".git"

var _ domPipeline.PipelineWorkdirRepository = (*PipelineWorkdirRepository)(nil)

type PipelineWorkdirRepository struct {
	workdirBasePath string
}

func NewPipelineWorkdirRepository(workdirBasePath string) domPipeline.PipelineWorkdirRepository {
	return &PipelineWorkdirRepository{workdirBasePath: workdirBasePath}
}

func (r *PipelineWorkdirRepository) destinationDir(projectUrl, pipelineUrl, environment string) string {
	projectName := utils.GetDirNameFromUrl(projectUrl)
	pipelineName := utils.GetDirNameFromUrl(pipelineUrl)
	return filepath.Join(r.workdirBasePath, projectName, "workdirs", pipelineName, environment)
}

func (r *PipelineWorkdirRepository) Copy(ctx *context.Context, localPipelinePath, projectUrl, pipelineUrl, environment string) (string, error) {
	sourceInfo, err := os.Stat(localPipelinePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("pipeline workdir repository: la ruta de origen no existe: %s", localPipelinePath)
		}
		return "", fmt.Errorf("pipeline workdir repository: stat origen '%s': %w", localPipelinePath, err)
	}
	if !sourceInfo.IsDir() {
		return "", fmt.Errorf("pipeline workdir repository: la ruta '%s' no es un directorio", localPipelinePath)
	}

	destDir := r.destinationDir(projectUrl, pipelineUrl, environment)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("pipeline workdir repository: crear directorio destino '%s': %w", destDir, err)
	}

	sourceMode := sourceInfo.Mode()

	err = filepath.WalkDir(localPipelinePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-(*ctx).Done():
			return (*ctx).Err()
		default:
		}

		if d.Name() == gitDirName {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(localPipelinePath, path)
		if err != nil {
			return fmt.Errorf("pipeline workdir repository: rel path '%s': %w", path, err)
		}
		if relPath == "." {
			return nil
		}

		destPath := filepath.Join(destDir, relPath)

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("pipeline workdir repository: info '%s': %w", path, err)
		}

		if d.IsDir() {
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				return fmt.Errorf("pipeline workdir repository: mkdir '%s': %w", destPath, err)
			}
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(destPath), sourceMode); err != nil {
			return fmt.Errorf("pipeline workdir repository: mkdir padre '%s': %w", filepath.Dir(destPath), err)
		}
		if err := copyFile(path, destPath, info.Mode()); err != nil {
			return fmt.Errorf("pipeline workdir repository: copiar '%s' -> '%s': %w", path, destPath, err)
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return destDir, nil
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
