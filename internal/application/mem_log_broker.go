package application

import (
	"sync"

	exePrt "github.com/jairoprogramador/vex-engine/internal/domain/execution/ports"
	exeVos "github.com/jairoprogramador/vex-engine/internal/domain/execution/vos"
)

const logChannelBuffer = 256

// MemLogBroker es un broker de logs en memoria que implementa ports.LogEmitter
// y permite a los callers suscribirse a los logs de una ejecución específica.
// Es thread-safe y pensado para SSE o cualquier consumidor de streaming.
type MemLogBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan string
}

// NewMemLogBroker construye un MemLogBroker listo para usar.
func NewMemLogBroker() *MemLogBroker {
	return &MemLogBroker{
		subscribers: make(map[string][]chan string),
	}
}

// Emit envía line a todos los canales suscritos al executionID dado.
// Implementa ports.LogEmitter. Las escrituras son non-blocking: si el buffer
// de un suscriptor está lleno, esa línea se descarta para ese suscriptor.
func (b *MemLogBroker) Emit(executionID exeVos.ExecutionID, line string) {
	b.mu.RLock()
	chans := b.subscribers[executionID.String()]
	b.mu.RUnlock()

	for _, ch := range chans {
		select {
		case ch <- line:
		default:
		}
	}
}

// Subscribe registra un nuevo suscriptor para executionID y retorna el canal
// desde el que recibirá líneas de log. El caller debe drenar el canal hasta
// que sea cerrado (ver Close).
func (b *MemLogBroker) Subscribe(executionID exeVos.ExecutionID) <-chan string {
	ch := make(chan string, logChannelBuffer)

	b.mu.Lock()
	b.subscribers[executionID.String()] = append(b.subscribers[executionID.String()], ch)
	b.mu.Unlock()

	return ch
}

// Close cierra y elimina todos los canales suscritos al executionID dado.
// Debe llamarse cuando la ejecución termina para que los consumidores salgan
// del range sobre el canal.
func (b *MemLogBroker) Close(executionID exeVos.ExecutionID) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := executionID.String()
	for _, ch := range b.subscribers[key] {
		close(ch)
	}
	delete(b.subscribers, key)
}

// Verificación de contrato en compile-time.
var _ exePrt.LogEmitter = (*MemLogBroker)(nil)
