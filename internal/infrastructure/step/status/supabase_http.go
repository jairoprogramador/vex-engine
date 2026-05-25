package status

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// supabasePost envía un payload JSON al endpoint con hasta maxAttempts intentos.
// Retorna el cuerpo de la respuesta exitosa o un error si todos fallan.
// El endpoint es una URL completa opaca: los repos no conocen el nombre de la función.
func supabasePost(
	client *http.Client,
	endpoint, token string,
	payload []byte,
	maxAttempts int,
) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(supabaseStatusRetryBackoff[attempt-1])
		}
		ctx, cancel := context.WithTimeout(context.Background(), supabaseStatusHTTPTimeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("build request: %w", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("do request: %w", err)
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return respBody, nil
		}
		lastErr = fmt.Errorf("http %d: %s", resp.StatusCode, string(respBody))
	}
	return nil, lastErr
}
