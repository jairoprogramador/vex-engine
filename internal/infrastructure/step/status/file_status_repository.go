package status

import (
	"errors"
	"fmt"

	domStepStatus "github.com/jairoprogramador/vex-engine/internal/domain/step/status"
)

var _ domStepStatus.StatusRepository = (*FileStatusRepository)(nil)

// FileStatusRepository elimina todos los archivos de estado persistidos relacionados con un paso:
// fingerprints de variables por paso/ambiente, tiempo, instrucciones de pipeline y proyecto (code).
type FileStatusRepository struct {
	vars domStepStatus.VariablesStatusRepository
	time domStepStatus.TimeStatusRepository
	inst domStepStatus.InstructionsStatusRepository
	code domStepStatus.CodeStatusRepository
}

func NewFileStatusRepository(
	vars domStepStatus.VariablesStatusRepository,
	timeStatus domStepStatus.TimeStatusRepository,
	inst domStepStatus.InstructionsStatusRepository,
	code domStepStatus.CodeStatusRepository,
) domStepStatus.StatusRepository {
	return &FileStatusRepository{
		vars: vars,
		time: timeStatus,
		inst: inst,
		code: code,
	}
}

func (r *FileStatusRepository) Delete(projectUrl, pipelineUrl, environment, step string) error {
	var errs []error

	if err := r.vars.Delete(projectUrl, pipelineUrl, environment, step); err != nil {
		errs = append(errs, fmt.Errorf("vars status: %w", err))
	}
	if err := r.time.Delete(projectUrl, environment, step); err != nil {
		errs = append(errs, fmt.Errorf("time status: %w", err))
	}
	if err := r.inst.Delete(projectUrl, pipelineUrl, step); err != nil {
		errs = append(errs, fmt.Errorf("instructions status: %w", err))
	}
	if err := r.code.Delete(projectUrl); err != nil {
		errs = append(errs, fmt.Errorf("code status: %w", err))
	}

	return errors.Join(errs...)
}
