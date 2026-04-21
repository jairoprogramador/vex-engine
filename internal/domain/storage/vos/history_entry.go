package vos

import "time"

// HistoryEntry es la interface mínima que una Rule necesita del aggregate.
// El aggregate concreto la implementa; así vos no importa aggregates.
type HistoryEntry interface {
	FindFingerprintByKind(kind FingerprintKind) (Fingerprint, bool)
	Environment() Environment
	CreatedAt() time.Time
}
