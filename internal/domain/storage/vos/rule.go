package vos

import "time"

// Rule es la interface del Specification Pattern aplicada a entradas históricas.
// Cada implementación captura una condición independiente que debe cumplirse
// para que una entrada histórica sea considerada válida (y el paso se omita).
//
// Se define en vos para que tanto aggregates como services/rules puedan importarla
// sin crear ciclos de dependencia.
type Rule interface {
	Satisfies(entry HistoryEntry, current FingerprintSet, now time.Time) bool
}
