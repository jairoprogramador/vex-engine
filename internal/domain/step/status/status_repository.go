package status

type StatusRepository interface {
	Delete(projectUrl, pipelineUrl, environment, step string) error
}
