package pipeline

import "context"

type PipelineWorkdirRepository interface {
	Copy(ctx *context.Context, localPipelinePath, projectUrl, pipelineUrl, environment string) (string, error)
}
