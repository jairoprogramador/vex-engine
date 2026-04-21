package vos

import (
	"testing"
	"time"
)

func TestTTL_DefaultOnZero(t *testing.T) {
	ttl := NewTTL(0)
	if ttl.Duration() != defaultTTLDuration {
		t.Errorf("se esperaba el default de 30d, obtuvo %v", ttl.Duration())
	}
}

func TestTTL_DefaultOnNegative(t *testing.T) {
	ttl := NewTTL(-1 * time.Hour)
	if ttl.Duration() != defaultTTLDuration {
		t.Errorf("se esperaba el default de 30d para negativo, obtuvo %v", ttl.Duration())
	}
}

func TestTTL_CustomValue(t *testing.T) {
	d := 7 * 24 * time.Hour
	ttl := NewTTL(d)
	if ttl.Duration() != d {
		t.Errorf("se esperaba %v, obtuvo %v", d, ttl.Duration())
	}
}
