package status

type CodeStatusRepository interface {
	Get(idProject string) (string, error)
	Set(idProject string, fingerprint string) error
	Delete(idProject string) error
}
