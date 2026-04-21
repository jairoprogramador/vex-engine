package storage

import "errors"

// ErrHistoryCorrupted se retorna cuando el artefacto de historial existe en disco
// pero no puede reconstruirse como VOs válidos. El ExecutionDecider lo trata como
// "sin historial" y decide Run, evitando falsos skip por datos corruptos.
var ErrHistoryCorrupted = errors.New("historial de ejecución corrupto")

// ErrUnknownStep se retorna cuando se solicita la política de un paso que no
// está registrado en el catálogo.
var ErrUnknownStep = errors.New("paso desconocido")
