package notify

import (
	"sync"

	"github.com/jairoprogramador/vex-engine/internal/domain/command"
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

const logChannelBuffer = 256

// MemLogPublisher es el sujeto (patrón Observer): recibe líneas desde el dominio,
// las reenvía a observadores externos (ej. StdoutLogObserver) y a suscriptores
// SSE mediante canales internos por executionID (Subscribe/Close permanece igual).
type MemLogPublisher struct {
	mu          sync.RWMutex
	observers   []domNotify.LogObserver
	subscribers map[string][]chan string
}

// NewMemLogPublisher construye un publicador sin observadores externos.
func NewMemLogPublisher() *MemLogPublisher {
	return &MemLogPublisher{
		subscribers: make(map[string][]chan string),
	}
}

// RegisterObserver añade un observador; se llamará tras cada Notify antes del SSE.
func (b *MemLogPublisher) RegisterObserver(o domNotify.LogObserver) {
	if o == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.observers = append(b.observers, o)
}

// Notify entrega una línea a todos los observadores registrados y luego al fan-out SSE
// (los canales creados con Subscribe para este executionID). Es idempotente con la
// firma domain/notify.LogObserver para poder inyectar el mismo *MemLogPublisher al ExecutionContext.
func (b *MemLogPublisher) Notify(executionID string, line string) {
	b.mu.RLock()
	copiedObservers := append([]domNotify.LogObserver(nil), b.observers...)
	chans := b.subscribers[executionID]
	b.mu.RUnlock()

	for _, o := range copiedObservers {
		o.Notify(executionID, line)
	}

	for _, ch := range chans {
		select {
		case ch <- line:
		default:
		}
	}
}

// Subscribe registra un suscriptor SSE para executionID y retorna el canal de líneas.
func (b *MemLogPublisher) Subscribe(executionID command.ExecutionID) <-chan string {
	ch := make(chan string, logChannelBuffer)

	b.mu.Lock()
	b.subscribers[executionID.String()] = append(b.subscribers[executionID.String()], ch)
	b.mu.Unlock()

	return ch
}

// Close cierra los canales SSE asociados a executionID.
func (b *MemLogPublisher) Close(executionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := executionID
	for _, ch := range b.subscribers[key] {
		close(ch)
	}
	delete(b.subscribers, key)
}

var _ domNotify.LogObserver = (*MemLogPublisher)(nil)
