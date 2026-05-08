package notify

import (
	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

// MultiObserver fan-out estático sobre N LogObserver. Reemplaza al MemLogPublisher
// como punto único de inyección al ExecutionContext: las cadenas de pipeline /
// step / command llaman a Notify y MultiObserver lo replica a cada observer
// concreto (StdoutLogObserver, SupabaseLogObserver, etc.).
//
// Las concreciones individuales son responsables de su propia sincronización
// interna; MultiObserver no introduce locking porque la lista de observers se
// fija al construir y no muta en runtime.
type MultiObserver struct {
	observers []domNotify.LogObserver
}

// NewMultiObserver descarta entradas nil — eso permite a los callers construir
// la lista condicionalmente sin filtrar.
func NewMultiObserver(observers ...domNotify.LogObserver) *MultiObserver {
	filtered := make([]domNotify.LogObserver, 0, len(observers))
	for _, o := range observers {
		if o != nil {
			filtered = append(filtered, o)
		}
	}
	return &MultiObserver{observers: filtered}
}

func (m *MultiObserver) Notify(executionID string, line string) {
	for _, o := range m.observers {
		o.Notify(executionID, line)
	}
}

// Close hace un best-effort de cerrar observers que implementen io.Closer-like
// (LogObserver no la define; la firma se mantiene para compat con quienes ya
// usan Close para flushear). Llamado desde RunCommand antes de reportar status
// terminal.
func (m *MultiObserver) Close() {
	for _, o := range m.observers {
		if c, ok := o.(interface{ Close() }); ok {
			c.Close()
		}
	}
}

var _ domNotify.LogObserver = (*MultiObserver)(nil)
