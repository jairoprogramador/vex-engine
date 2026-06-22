package status

type CodeStatusRepository interface {
	Get(idProject, idPipeline, idStep string) (string, error)
	Set(idProject, idPipeline, idStep, fingerprint string) error
	Delete(idProject, idPipeline, idStep string) error
}
