package ports

import "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"

// LogEmitter define el contrato para emitir líneas de log asociadas a una ejecución.
// Las implementaciones pueden enviar a un buffer en memoria, a un websocket,
// a un archivo, etc.
type LogEmitter interface {
	Emit(executionID vos.ExecutionID, line string)
}
