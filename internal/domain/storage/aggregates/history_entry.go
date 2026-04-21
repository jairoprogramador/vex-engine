package aggregates

import (
	"time"

	"github.com/jairoprogramador/vex-engine/internal/domain/storage/vos"
)

// HistoryEntry registra el estado de un paso en un momento dado.
// No tiene setter SetCreatedAt — la construcción es la única vía de asignar el tiempo.
type HistoryEntry struct {
	fingerprints map[vos.FingerprintKind]vos.Fingerprint
	environment  vos.Environment
	createdAt    time.Time
}

// NewHistoryEntry construye una entrada con el timestamp actual UTC.
func NewHistoryEntry(set vos.FingerprintSet, at time.Time) HistoryEntry {
	fps := make(map[vos.FingerprintKind]vos.Fingerprint)
	for _, kind := range []vos.FingerprintKind{vos.KindCode, vos.KindInstruction, vos.KindVars} {
		if fp, ok := set.Get(kind); ok {
			fps[kind] = fp
		}
	}
	return HistoryEntry{
		fingerprints: fps,
		environment:  set.Environment(),
		createdAt:    at.UTC(),
	}
}

// FindFingerprintByKind retorna el fingerprint para el kind dado y si existe.
// Implementa vos.HistoryEntry.
func (e HistoryEntry) FindFingerprintByKind(kind vos.FingerprintKind) (vos.Fingerprint, bool) {
	fp, ok := e.fingerprints[kind]
	return fp, ok
}

// Environment retorna el entorno registrado. Implementa vos.HistoryEntry.
func (e HistoryEntry) Environment() vos.Environment {
	return e.environment
}

// CreatedAt retorna el momento de creación. Implementa vos.HistoryEntry.
func (e HistoryEntry) CreatedAt() time.Time {
	return e.createdAt
}

func (e HistoryEntry) Equals(other HistoryEntry) bool {
	if !e.environment.Equals(other.environment) {
		return false
	}
	if !e.createdAt.Equal(other.createdAt) {
		return false
	}
	if len(e.fingerprints) != len(other.fingerprints) {
		return false
	}
	for kind, fp := range e.fingerprints {
		otherFp, ok := other.fingerprints[kind]
		if !ok || !fp.Equals(otherFp) {
			return false
		}
	}
	return true
}

// HistoryEntry implementa vos.HistoryEntry en compile-time.
var _ vos.HistoryEntry = HistoryEntry{}
