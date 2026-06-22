package status

import "time"

// ── Constantes para los repositorios de estado locales (archivo GOB) ─────────

const (
	statusDirName      = "status"
	varsStatusFileName = "vars.status"
)

// ── Constantes para los repositorios de estado remotos (Supabase) ────────────

const (
	supabaseStatusHTTPTimeout  = 10 * time.Second
	supabaseStatusGetRetries   = 2
	supabaseStatusWriteRetries = 3
)

// supabaseStatusRetryBackoff es la pausa entre reintentos consecutivos.
// Índice 0 → pausa antes del intento 1, índice 1 → antes del intento 2, etc.
var supabaseStatusRetryBackoff = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}
