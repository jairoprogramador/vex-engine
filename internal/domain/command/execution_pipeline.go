package command

type ExecutionPipeline struct {
	pipelineUrl       string
	pipelineRef       string
	pipelineLocalPath string
}

func NewExecutionPipeline(pipelineUrl, pipelineRef string) ExecutionPipeline {
	return ExecutionPipeline{
		pipelineUrl:       pipelineUrl,
		pipelineRef:       pipelineRef,
		pipelineLocalPath: "",
	}
}

func (e *ExecutionPipeline) PipelineUrl() string {
	return e.pipelineUrl
}

func (e *ExecutionPipeline) PipelineRef() string {
	return e.pipelineRef
}

func (e *ExecutionPipeline) SetPipelineLocalPath(pipelineLocalPath string) {
	e.pipelineLocalPath = pipelineLocalPath
}

func (e *ExecutionPipeline) PipelineLocalPath() string {
	return e.pipelineLocalPath
}
