package storage

import (
	"time"

	storagePorts "github.com/jairoprogramador/vex-engine/internal/domain/storage/ports"
)

type SystemClock struct{}

func NewSystemClock() storagePorts.Clock {
	return &SystemClock{}
}

func (c *SystemClock) Now() time.Time {
	return time.Now().UTC()
}

var _ storagePorts.Clock = (*SystemClock)(nil)
