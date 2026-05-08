package status

type VariablesStatusRepository interface {
	Get(projectUrl, pipelineUrl, environment, step string) (string, error)
	Set(projectUrl, pipelineUrl, environment, step string, fingerprint string) error
	Delete(projectUrl, pipelineUrl, environment, step string) error
}
