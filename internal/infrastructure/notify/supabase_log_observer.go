package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	// supabaseMaxBatchLines es el máximo absoluto de líneas que el endpoint
	// log-ingest acepta por POST (§6.6 plan_deploy.md).
	supabaseMaxBatchLines = 200
	// supabaseMaxBatchBytes es el límite total de payload por POST. Si el buffer
	// crece más, se parte en múltiples flushes consecutivos.
	supabaseMaxBatchBytes = 64 * 1024
	// supabaseMaxLineBytes acota cada línea individualmente: el plan especifica
	// 64KB total por batch, así que una línea sola que exceda ese tamaño es
	// inválida. Truncamos con sufijo claro antes de encolarla.
	supabaseMaxLineBytes = supabaseMaxBatchBytes - 256
	// supabaseTruncationSuffix se concatena a líneas excesivas tras el corte.
	supabaseTruncationSuffix = "...[truncated]"
	// supabaseLineChannelCapacity dimensiona el buffer del productor. Un canal
	// lleno indica back-pressure: el observer marca logsLost en lugar de bloquear
	// la pipeline.
	supabaseLineChannelCapacity = 1024
	// supabaseHTTPTimeout limita el tiempo de cada POST individual.
	supabaseHTTPTimeout = 5 * time.Second
)

// supabaseLogStream es el valor del campo `stream` del payload §6.6. vexd no
// distingue stdout/stderr a nivel ExecutionContext.Emit (un único canal lógico
// para todas las líneas), así que reportamos siempre "stdout". El campo se
// mantiene para compatibilidad futura cuando el dominio gane esa distinción.
const supabaseLogStream = "stdout"

// supabaseRetryBackoff es la secuencia de esperas entre reintentos del POST de
// un batch. 3 intentos totales: el inicial + dos retries con backoffs 500ms/1s/2s
// (la última pausa se duplica al ser el último intento).
var supabaseRetryBackoff = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}

// logLine es la unidad encolada por Notify y consumida por la goroutine de
// flush. seq se asigna de forma monotónica por observer.
type logLine struct {
	seq  int64
	line string
}

// SupabaseLogObserver implementa domNotify.LogObserver enviando los logs en
// batches a la edge function `log-ingest`. La forma del struct, la firma de
// NewSupabaseLogObserver y la semántica de Close/LogsLost son las definitivas
// (heredadas del stub M3); M5 conecta la goroutine de flush + cliente HTTP.
//
// Diseño:
//
//   - Notify es no-bloqueante: encola en un canal de capacidad
//     supabaseLineChannelCapacity. Si el canal está lleno (consumer no
//     drenando lo suficientemente rápido), descarta la línea y marca
//     logsLost. Esto preserva la prioridad: la pipeline NUNCA se ralentiza
//     por el observer.
//
//   - Una sola goroutine de flush despierta cada 200ms o cuando el buffer
//     interno alcanza 50 líneas (lo que ocurra antes). POST con retry 3×
//     backoff 500ms/1s/2s; si todos los intentos fallan se marca logsLost
//     y el batch se descarta (no reintentamos batches viejos: priorizamos
//     no acumular memoria sobre garantías de entrega).
//
//   - Close drena el canal pendiente, hace un flush final y espera al
//     término de la goroutine. Idempotente: llamadas posteriores son no-op.
type SupabaseLogObserver struct {
	httpClient  *http.Client
	endpoint    string
	token       string
	executionID string

	lines    chan logLine
	stop     chan struct{}
	wg       sync.WaitGroup
	closed   atomic.Bool
	seq      atomic.Int64
	logsLost atomic.Bool
}

// NewSupabaseLogObserver construye el observer con el endpoint y el bearer
// token. La goroutine de flush se arranca inmediatamente y se queda esperando
// líneas; en modo local (sin endpoint) los callers no construyen el observer.
func NewSupabaseLogObserver(endpoint, token, executionID string) *SupabaseLogObserver {
	o := &SupabaseLogObserver{
		httpClient:  &http.Client{Timeout: supabaseHTTPTimeout},
		endpoint:    endpoint,
		token:       token,
		executionID: executionID,
		lines:       make(chan logLine, supabaseLineChannelCapacity),
		stop:        make(chan struct{}),
	}
	o.wg.Add(1)
	go o.flushLoop()
	return o
}

// Notify enqueues a line for batched POST. Non-blocking: a full channel is
// reported as logsLost rather than blocking the pipeline. The seq counter is
// assigned at enqueue time so order matches the order of Emit calls (the
// channel preserves FIFO for a single producer; vexd emits sequentially).
func (o *SupabaseLogObserver) Notify(executionID string, line string) {
	if o.closed.Load() {
		// Tras Close(), nuevos Notify se descartan silenciosamente: la
		// pipeline ya terminó y el flush final está corriendo o se encoló.
		return
	}

	payload := truncateLine(line)
	seq := o.seq.Add(1)

	select {
	case o.lines <- logLine{seq: seq, line: payload}:
		// encolado.
	default:
		// Buffer saturado: el batcher no drena lo bastante rápido. Marcamos
		// logsLost; el reporter terminal lo incluirá en el payload final.
		o.logsLost.Store(true)
	}

	_ = executionID // el contrato del LogObserver lo recibe; el observer usa o.executionID almacenado.
}

// Close marca el observer como cerrado, drena el buffer pendiente con un flush
// final y espera al término de la goroutine. Llamarlo una sola vez antes de
// reportar el status terminal: nuevas líneas tras Close se descartan.
func (o *SupabaseLogObserver) Close() {
	if !o.closed.CompareAndSwap(false, true) {
		return
	}
	close(o.stop)
	o.wg.Wait()
}

// LogsLost indica si al menos un batch agotó sus retries o si una línea fue
// descartada por canal lleno. SupabaseStatusReporter lo lee al construir el
// payload terminal.
func (o *SupabaseLogObserver) LogsLost() bool {
	return o.logsLost.Load()
}

// flushLoop es el consumidor único del canal lines. Vacía el buffer cada
// supabaseFlushInterval o cuando alcanza supabaseFlushSize, lo que ocurra
// primero. Termina cuando stop está cerrado y el canal está vacío.
func (o *SupabaseLogObserver) flushLoop() {
	defer o.wg.Done()

	ticker := time.NewTicker(supabaseFlushInterval)
	defer ticker.Stop()

	buf := make([]logLine, 0, supabaseFlushSize)

	for {
		select {
		case <-o.stop:
			// Consumir lo que quede sin bloquear y enviar un último batch.
			for {
				select {
				case ln := <-o.lines:
					buf = append(buf, ln)
				default:
					o.flushAll(buf)
					return
				}
			}

		case ln := <-o.lines:
			buf = append(buf, ln)
			if len(buf) >= supabaseFlushSize {
				o.flushAll(buf)
				buf = buf[:0]
			}

		case <-ticker.C:
			if len(buf) > 0 {
				o.flushAll(buf)
				buf = buf[:0]
			}
		}
	}
}

// flushAll envía el slice completo respetando los límites de tamaño del
// endpoint (200 líneas / 64KB por batch). Para batches grandes lo divide en
// fragmentos consecutivos. Cualquier error termina marcando logsLost; no
// reencolamos para evitar acumulación de memoria.
func (o *SupabaseLogObserver) flushAll(batch []logLine) {
	if len(batch) == 0 {
		return
	}

	start := 0
	for start < len(batch) {
		end, payload, err := o.buildBatch(batch, start)
		if err != nil {
			fmt.Fprintf(os.Stderr, "vexd: log observer build batch: %v\n", err)
			o.logsLost.Store(true)
			return
		}
		if err := o.postWithRetry(payload); err != nil {
			fmt.Fprintf(os.Stderr, "vexd: log observer flush (lines %d..%d): %v\n",
				batch[start].seq, batch[end-1].seq, err)
			o.logsLost.Store(true)
		}
		start = end
	}
}

// buildBatch toma el slice global y devuelve el rango [start, end) que cabe
// dentro de los límites del endpoint, junto con el payload JSON ya serializado.
// Garantiza progreso: si una sola línea ya excede el límite, igual la incluye
// (tras truncateLine es imposible que exceda supabaseMaxBatchBytes para una
// sola entrada porque truncateLine ya recorta a supabaseMaxLineBytes).
func (o *SupabaseLogObserver) buildBatch(batch []logLine, start int) (int, []byte, error) {
	type batchLine struct {
		Seq    int64  `json:"seq"`
		Stream string `json:"stream"`
		Line   string `json:"line"`
	}
	type batchPayload struct {
		ExecutionID string      `json:"execution_id"`
		Lines       []batchLine `json:"lines"`
	}

	end := start
	// Primera estimación de líneas: cap por count.
	countCap := start + supabaseMaxBatchLines
	if countCap > len(batch) {
		countCap = len(batch)
	}

	// Envelope payload bytes ya gastados por estructura JSON sin lines.
	const overheadEstimate = 64
	totalBytes := overheadEstimate
	for end < countCap {
		// Aproximación: longitud de la línea + bytes de envoltorio por entrada.
		lineCost := len(batch[end].line) + 80 // seq, stream, JSON braces, commas, escapes margen.
		if end > start && totalBytes+lineCost > supabaseMaxBatchBytes {
			break
		}
		totalBytes += lineCost
		end++
	}

	if end == start {
		// Defensive: garantizamos al menos una línea por batch.
		end = start + 1
	}

	out := batchPayload{ExecutionID: o.executionID, Lines: make([]batchLine, 0, end-start)}
	for i := start; i < end; i++ {
		out.Lines = append(out.Lines, batchLine{
			Seq:    batch[i].seq,
			Stream: supabaseLogStream,
			Line:   batch[i].line,
		})
	}

	body, err := json.Marshal(out)
	if err != nil {
		return 0, nil, fmt.Errorf("marshal batch: %w", err)
	}
	return end, body, nil
}

// postWithRetry intenta enviar el batch hasta len(supabaseRetryBackoff) veces.
// Backoff entre intento i y i+1: supabaseRetryBackoff[i]. El primer intento es
// inmediato.
func (o *SupabaseLogObserver) postWithRetry(payload []byte) error {
	if o.endpoint == "" {
		return fmt.Errorf("log observer endpoint not configured")
	}

	var lastErr error
	for attempt := 0; attempt < len(supabaseRetryBackoff); attempt++ {
		if attempt > 0 {
			time.Sleep(supabaseRetryBackoff[attempt-1])
		}

		ctx, cancel := context.WithTimeout(context.Background(), supabaseHTTPTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint, bytes.NewReader(payload))
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("build request: %w", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if o.token != "" {
			req.Header.Set("Authorization", "Bearer "+o.token)
		}

		resp, err := o.httpClient.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("post: %w", err)
			continue
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		cancel()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		lastErr = fmt.Errorf("post returned status %d", resp.StatusCode)
	}
	return lastErr
}

// truncateLine recorta líneas cuyo tamaño individual amenazaría el batch
// completo. Sin esto, una línea de 1MB haría que `buildBatch` no progresara.
func truncateLine(line string) string {
	if len(line) <= supabaseMaxLineBytes {
		return line
	}
	cut := supabaseMaxLineBytes - len(supabaseTruncationSuffix)
	if cut < 0 {
		cut = 0
	}
	return line[:cut] + supabaseTruncationSuffix
}

var _ domNotify.LogObserver = (*SupabaseLogObserver)(nil)
