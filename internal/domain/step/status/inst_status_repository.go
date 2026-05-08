package status

type InstructionsStatusRepository interface {
	Get(projectUrl, pipelineUrl, step string) (string, error)
	Set(projectUrl, pipelineUrl, step, fingerprint string) error
	Delete(projectUrl, pipelineUrl, step string) error
}
