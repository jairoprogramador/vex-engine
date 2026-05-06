package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// receivedBatch refleja el shape del payload esperado por log-ingest. Los
// tests lo deserializan para verificar contenido y orden de los seq.
type receivedBatch struct {
	ExecutionID string `json:"execution_id"`
	Lines       []struct {
		Seq    int64  `json:"seq"`
		Stream string `json:"stream"`
		Line   string `json:"line"`
	} `json:"lines"`
}

func TestSupabaseLogObserver_Notify_BatchesAndPosts(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		batches []receivedBatch
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer t0k3n" {
			t.Errorf("authorization header: got %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var batch receivedBatch
		if err := json.Unmarshal(body, &batch); err != nil {
			t.Errorf("decode body: %v", err)
		}
		mu.Lock()
		batches = append(batches, batch)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	observer := NewSupabaseLogObserver(srv.URL, "t0k3n")
	observer.httpClient = srv.Client()

	for i := 0; i < 5; i++ {
		observer.Notify("exec-1", "line-"+itoa(i))
	}

	observer.Close()

	mu.Lock()
	defer mu.Unlock()
	if len(batches) == 0 {
		t.Fatalf("expected at least one batch, got 0")
	}
	var seqs []int64
	for _, b := range batches {
		for _, ln := range b.Lines {
			if ln.Stream != "stdout" {
				t.Errorf("unexpected stream %q", ln.Stream)
			}
			seqs = append(seqs, ln.Seq)
		}
	}
	if len(seqs) != 5 {
		t.Fatalf("expected 5 lines across batches, got %d (batches=%+v)", len(seqs), batches)
	}
	for i, s := range seqs {
		if s != int64(i+1) {
			t.Fatalf("seq[%d]: got %d, want %d", i, s, i+1)
		}
	}
	if observer.LogsLost() {
		t.Fatalf("LogsLost should be false on happy path")
	}
}

func TestSupabaseLogObserver_FlushSizeForcesBatch(t *testing.T) {
	t.Parallel()

	var calls atomic.Int64
	totalLines := atomic.Int64{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var batch receivedBatch
		if err := json.Unmarshal(body, &batch); err == nil {
			totalLines.Add(int64(len(batch.Lines)))
		}
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	observer := NewSupabaseLogObserver(srv.URL, "")
	observer.httpClient = srv.Client()

	// Genera líneas en bursts pequeños separados por una pausa < flushInterval
	// para que la goroutine alcance a procesar y dispare el flush por tamaño.
	for burst := 0; burst < 3; burst++ {
		for i := 0; i < supabaseFlushSize; i++ {
			observer.Notify("exec-1", "line")
		}
		time.Sleep(20 * time.Millisecond)
	}
	observer.Close()

	if got := totalLines.Load(); got != int64(supabaseFlushSize*3) {
		t.Fatalf("total lines delivered: got %d, want %d", got, supabaseFlushSize*3)
	}
	if got := calls.Load(); got < 2 {
		t.Fatalf("expected multiple flushes, got %d POSTs", got)
	}
}

func TestSupabaseLogObserver_RetriesOnServerError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		n := attempts.Add(1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Acotamos el backoff para que el test sea rápido sin perder semántica.
	prevBackoff := supabaseRetryBackoff
	supabaseRetryBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { supabaseRetryBackoff = prevBackoff }()

	observer := NewSupabaseLogObserver(srv.URL, "")
	observer.httpClient = srv.Client()

	observer.Notify("exec-1", "hello")
	observer.Close()

	if got := attempts.Load(); got < 3 {
		t.Fatalf("expected at least 3 POST attempts, got %d", got)
	}
	if observer.LogsLost() {
		t.Fatalf("LogsLost should be false: third attempt succeeded")
	}
}

func TestSupabaseLogObserver_LogsLostWhenAllRetriesFail(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	prevBackoff := supabaseRetryBackoff
	supabaseRetryBackoff = []time.Duration{1 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond}
	defer func() { supabaseRetryBackoff = prevBackoff }()

	observer := NewSupabaseLogObserver(srv.URL, "")
	observer.httpClient = srv.Client()

	observer.Notify("exec-1", "doomed")
	observer.Close()

	if !observer.LogsLost() {
		t.Fatalf("LogsLost should be true when all retries fail")
	}
}

func TestSupabaseLogObserver_CloseIsIdempotent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	observer := NewSupabaseLogObserver(srv.URL, "")
	observer.httpClient = srv.Client()
	observer.Notify("exec-1", "x")
	observer.Close()
	// Una segunda llamada debe ser no-op (no panic, no double-close del canal).
	observer.Close()
	// Notify después de Close se descarta silenciosamente.
	observer.Notify("exec-1", "y")
}

func TestSupabaseLogObserver_TruncatesOversizedLines(t *testing.T) {
	t.Parallel()

	var receivedLine string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var batch receivedBatch
		if err := json.Unmarshal(body, &batch); err == nil && len(batch.Lines) > 0 {
			receivedLine = batch.Lines[0].Line
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	observer := NewSupabaseLogObserver(srv.URL, "")
	observer.httpClient = srv.Client()

	huge := strings.Repeat("A", supabaseMaxBatchBytes*2)
	observer.Notify("exec-1", huge)
	observer.Close()

	if !strings.HasSuffix(receivedLine, supabaseTruncationSuffix) {
		t.Fatalf("expected truncation suffix; got len=%d suffix=%q",
			len(receivedLine), tailString(receivedLine, len(supabaseTruncationSuffix)))
	}
	if len(receivedLine) > supabaseMaxLineBytes {
		t.Fatalf("truncated line still too long: %d > %d", len(receivedLine), supabaseMaxLineBytes)
	}
}

// itoa minimal sin importar strconv solo por test. Evita anillado innecesario.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func tailString(s string, n int) string {
	if n >= len(s) {
		return s
	}
	return s[len(s)-n:]
}
