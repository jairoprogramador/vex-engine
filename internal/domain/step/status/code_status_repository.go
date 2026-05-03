package status

type CodeStatusRepository interface {
	Get(projectUrl string) (string, error)
	Set(projectUrl string, fingerprint string) error
	Delete(projectUrl string) error
}
