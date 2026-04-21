package vos

import "errors"

type Fingerprint struct {
	value string
}

func NewFingerprint(value string) (Fingerprint, error) {
	if value == "" {
		return Fingerprint{}, errors.New("el valor del fingerprint no puede estar vacío")
	}
	return Fingerprint{value: value}, nil
}

func (f Fingerprint) String() string {
	return f.value
}

func (f Fingerprint) Equals(other Fingerprint) bool {
	return f.value == other.value
}
