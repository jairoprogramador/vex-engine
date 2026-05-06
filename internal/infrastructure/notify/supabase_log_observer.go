package notify

import (
	"sync"
	"sync/atomic"
	"time"

	domNotify "github.com/jairoprogramador/vex-engine/internal/domain/notify"
)

const (
	// supabaseFlushInterval define cuándo se vacía el buffer aunque no esté lleno.
	supabaseFlushInterval = 200 * time.Millisecond
	// supabaseFlushSize define cuántas líneas fuerzan un flush antes del intervalo.
	supabaseFlushSize = 50
)

// SupabaseLogObserver es el observador que en M5 enviará logs en batch a la
// edge function `log-ingest` de Supabase. En M3 cumple la interfaz LogObserver
// y mantiene el contador `seq` y la bandera `LogsLost`, pero NO realiza POST:
// los logs se descartan silenciosamente. La forma del struct, la firma de
// NewSupabaseLogObserver y la semántica de Close son las definitivas; M5
// añadirá la goroutine de flush + retry sin romper el wiring del RunCommand.
//
// El campo LogsLost se expone para que SupabaseStatusReporter pueda incluirlo
// en el payload terminal (`logs_lost` true cuando todos los retries fallaron).
type SupabaseLogObserver struct {
	endpoint string
	token    string

	mu     sync.Mutex
	seq    uint64
	closed bool

	// logsLost se setea en true cuando un batch falla todos sus retries (M5).
	// En M3 permanece false porque no hay POST.
	logsLost atomic.Bool
}

// NewSupabaseLogObserver construye el observer con el endpoint y el bearer token.
// En M3 los argumentos se almacenan pero no se usan; en M5 se conectan al cliente HTTP.
func NewSupabaseLogObserver(endpoint, token string) *SupabaseLogObserver {
	return &SupabaseLogObserver{
		endpoint: endpoint,
		token:    token,
	}
}

func (o *SupabaseLogObserver) Notify(executionID string, line string) {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return
	}
	o.seq++
	o.mu.Unlock()

	// M3: no-op. M5 encolará { execution_id, seq, stream, line } en el buffer.
	_ = executionID
	_ = line
}

// Close marca el observer como cerrado y, en M5, esperará a que el buffer se
// vacíe (timeout: supabaseFlushInterval * 3). En M3 solo flippea el flag.
func (o *SupabaseLogObserver) Close() {
	o.mu.Lock()
	o.closed = true
	o.mu.Unlock()
}

// LogsLost indica si al menos un batch perdió todos sus retries durante la
// ejecución. SupabaseStatusReporter lo lee al construir el payload terminal.
func (o *SupabaseLogObserver) LogsLost() bool {
	return o.logsLost.Load()
}

var _ domNotify.LogObserver = (*SupabaseLogObserver)(nil)
