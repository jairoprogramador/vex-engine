package status

type InstructionsStatusRepository interface {
	Get(idProject, idPipeline, idStep string) (string, error)
	Set(idProject, idPipeline, idStep, fingerprint string) error
	Delete(idProject, idPipeline, idStep string) error
}
