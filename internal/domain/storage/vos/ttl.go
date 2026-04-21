package vos

import "time"

const defaultTTLDuration = 30 * 24 * time.Hour

// TTL representa la duración de validez de una entrada histórica de ejecución.
// Un TTL zero-value no es válido; usar NewTTL(0) para obtener el default de 30 días.
type TTL struct {
	duration time.Duration
}

// NewTTL crea un TTL. Si d <= 0 se aplica el default de 30 días.
func NewTTL(d time.Duration) TTL {
	if d <= 0 {
		d = defaultTTLDuration
	}
	return TTL{duration: d}
}

func (t TTL) Duration() time.Duration {
	return t.duration
}
