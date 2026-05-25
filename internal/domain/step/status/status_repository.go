package status

type StatusRepository interface {
	Delete(idProject, idPipeline, idEnvironment, idStep string) error
}
