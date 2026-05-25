package status

import "time"

type TimeStatusRepository interface {
	Get(idProject, idEnvironment, idStep string) (time.Time, error)
	Set(idProject, idEnvironment, idStep string, time time.Time) error
	Delete(idProject, idEnvironment, idStep string) error
}
