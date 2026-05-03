package status

import "time"

type TimeStatusRepository interface {
	Get(projectUrl, environment, step string) (time.Time, error)
	Set(projectUrl, environment, step string, time time.Time) error
	Delete(projectUrl, environment, step string) error
}
