package status

type VariablesStatusRepository interface {
	Get(idProject, idPipeline, idEnvironment, idStep string) (string, error)
	Set(idProject, idPipeline, idEnvironment, idStep string, fingerprint string) error
	Delete(idProject, idPipeline, idEnvironment, idStep string) error
}
