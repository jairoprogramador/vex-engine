package mappers

/*
import (
	dtoAppl "github.com/jairoprogramador/vex-engine/internal/application/dto"

	execAggr "github.com/jairoprogramador/vex-engine/internal/domain/execution/handlers"

	pipeDoma "github.com/jairoprogramador/vex-engine/internal/domain/pipeline"

	projDoma "github.com/jairoprogramador/vex-engine/internal/domain/project"

	iUtils "github.com/jairoprogramador/vex-engine/internal/infrastructure/utils"
)

func MapToExecutionVariables(pipelineVariables []pipeDoma.Variable) (execAggr.VariablesMap, error) {
	executionVariables := execAggr.NewVariablesMap()
	for _, pipelineVariable := range pipelineVariables {
		outputVar, err := execAggr.NewVariable(pipelineVariable.Name(), pipelineVariable.Value(), false)
		if err != nil {
			return execAggr.NewVariablesMap(), err
		}
		executionVariables.Add(outputVar)
	}
	return executionVariables, nil
}

func MapToExecutionCommands(pipelineCommands []pipeDoma.Command) ([]execAggr.Command, error) {
	executionCommands := make([]execAggr.Command, 0, len(pipelineCommands))
	for _, pipelineCommand := range pipelineCommands {
		executionOutputs, err := MapToExecutionOutputs(pipelineCommand.Outputs())
		if err != nil {
			return nil, err
		}

		executionCommand, err := execAggr.NewCommand(
			pipelineCommand.Name(),
			pipelineCommand.Cmd(),
			execAggr.WithTemplatePaths(pipelineCommand.TemplateFiles()),
			execAggr.WithWorkdir(pipelineCommand.Workdir()),
			execAggr.WithOutputs(executionOutputs),
		)
		if err != nil {
			return nil, err
		}
		executionCommands = append(executionCommands, executionCommand)
	}

	return executionCommands, nil
}

func MapToExecutionOutputs(pipelineOutputs []pipeDoma.Output) ([]execAggr.CommandOutput, error) {
	if len(pipelineOutputs) == 0 {
		return nil, nil
	}

	execOutputs := make([]execAggr.CommandOutput, 0, len(pipelineOutputs))
	for _, pipelineOutput := range pipelineOutputs {
		output, err := execAggr.NewCommandOutput(
			pipelineOutput.Name(),
			pipelineOutput.Probe(),
		)
		if err != nil {
			return nil, err
		}
		execOutputs = append(execOutputs, output)
	}
	return execOutputs, nil
}

func MapToExecution(request dtoAppl.RequestInput) (*execAggr.Execution, error) {
	runtime := execAggr.NewRuntime(request.Execution.RuntimeImage, request.Execution.RuntimeTag)

	execution := execAggr.NewExecution(
		request.Project.Id,
		request.Project.Name,
		request.Pipeline.Url,
		request.Pipeline.Ref,
		request.Execution.Step,
		request.Execution.Environment,
		runtime,
	)
	return execution, nil
}

func MapToStepExecution(
	pipelineStep *pipeDoma.Step,
	environmentValue string,
	projectUrl projDoma.ProjectUrl,
	pipelineUrl pipeDoma.PipelineURL,
	pipelinesBasePath string,
	workdirBasePath string,
	storageBasePath string,
) (execAggr.StepExecution, error) {
	executionCommands, err := MapToExecutionCommands(pipelineStep.Commands())
	if err != nil {
		return execAggr.StepExecution{}, err
	}

	executionVars, err := MapToExecutionVariables(pipelineStep.Variables())
	if err != nil {
		return execAggr.StepExecution{}, err
	}

	return execAggr.StepExecution{
		Name:                    pipelineStep.StepName().Name(),
		Commands:                executionCommands,
		PipelineVars:            executionVars,
		PipelinePath:            iUtils.LocalRepositoryPath(pipelinesBasePath, pipelineUrl),
		EnvironmentWorkdirPath:  iUtils.EnvironmentWorkdirPath(workdirBasePath, environmentValue, pipelineStep.StepName().Name(), projectUrl, pipelineUrl),
		SharedWorkdirPath:       iUtils.SharedWorkdirPath(workdirBasePath, pipelineStep.StepName().Name(), projectUrl, pipelineUrl),
		VarsEnvironmentFilePath: iUtils.VarsExecutionFilePath(storageBasePath, environmentValue, pipelineStep.StepName().Name(), projectUrl, pipelineUrl),
		VarsSharedFilePath:      iUtils.VarsExecutionFilePath(storageBasePath, execAggr.SharedScopeName, pipelineStep.StepName().Name(), projectUrl, pipelineUrl),
	}, nil
}
*/
