package application

import (
	"fmt"

	execEnt "github.com/jairoprogramador/vex-engine/internal/domain/execution/entities"
	execVos "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
	dom "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"
)

func mapToExecutionStep(defStep *dom.Step, workspaceStep, workspaceShared string) (*execEnt.Step, error) {
	execCmds, err := mapToExecutionCommands(defStep.Commands())
	if err != nil {
		return nil, fmt.Errorf("error al mapear los comandos para el paso '%s': %w", defStep.Name().Name(), err)
	}

	execVars, err := mapToExecutionVariables(defStep.Variables())
	if err != nil {
		return nil, fmt.Errorf("error al mapear las variables para el paso '%s': %w", defStep.Name().Name(), err)
	}

	execStep, err := execEnt.NewStep(
		defStep.Name().Name(),
		execEnt.WithCommands(execCmds),
		execEnt.WithVariables(execVars),
		execEnt.WithWorkspaceStep(workspaceStep),
		execEnt.WithWorkspaceShared(workspaceShared),
	)
	if err != nil {
		return nil, fmt.Errorf("error al crear el paso de ejecución para '%s': %w", defStep.Name().Name(), err)
	}

	return &execStep, nil
}

func mapToExecutionVariables(defVars []dom.Variable) (execVos.VariableSet, error) {
	execVars := execVos.NewVariableSet()
	for _, defVar := range defVars {
		outputVar, err := execVos.NewOutputVar(defVar.Name(), defVar.Value(), false)
		if err != nil {
			return execVos.NewVariableSet(), err
		}
		execVars.Add(outputVar)
	}
	return execVars, nil
}

func mapToExecutionCommands(defCmds []dom.Command) ([]execVos.Command, error) {
	execCmds := make([]execVos.Command, 0, len(defCmds))
	for _, defCmd := range defCmds {
		cmdOutputs, err := mapToExecutionOutputs(defCmd.Outputs())
		if err != nil {
			return nil, err
		}

		execCmd, err := execVos.NewCommand(
			defCmd.Name(),
			defCmd.Cmd(),
			execVos.WithTemplateFiles(defCmd.TemplateFiles()),
			execVos.WithWorkdir(defCmd.Workdir()),
			execVos.WithOutputs(cmdOutputs),
		)
		if err != nil {
			return nil, err
		}
		execCmds = append(execCmds, execCmd)
	}

	return execCmds, nil
}

func mapToExecutionOutputs(defOutputs []dom.Output) ([]execVos.CommandOutput, error) {
	if len(defOutputs) == 0 {
		return nil, nil
	}

	execOutputs := make([]execVos.CommandOutput, 0, len(defOutputs))
	for _, defOutput := range defOutputs {
		output, err := execVos.NewCommandOutput(
			defOutput.Name(),
			defOutput.Probe(),
		)
		if err != nil {
			return nil, err
		}
		execOutputs = append(execOutputs, output)
	}
	return execOutputs, nil
}
