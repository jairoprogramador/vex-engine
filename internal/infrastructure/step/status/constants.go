package status

import "time"

// ── Constantes para los repositorios de estado locales (archivo GOB) ─────────

const (
	statusDirName      = "status"
	varsStatusFileName = "vars.status"
	codeStatusFileName = "code.status"
)

// ── Constantes para los repositorios de estado remotos (Supabase) ────────────

const (
	// supabaseStatusHTTPTimeout limita el tiempo de cada POST a las edge functions.
	supabaseStatusHTTPTimeout = 10 * time.Second

	// supabaseStatusGetRetries es el número máximo de intentos para operaciones Get.
	supabaseStatusGetRetries = 2

	// supabaseStatusWriteRetries es el número máximo de intentos para Set y Delete.
	supabaseStatusWriteRetries = 3
)

// supabaseStatusRetryBackoff es la pausa entre reintentos consecutivos.
// Índice 0 → pausa antes del intento 1, índice 1 → antes del intento 2, etc.
var supabaseStatusRetryBackoff = []time.Duration{
	500 * time.Millisecond,
	1 * time.Second,
	2 * time.Second,
}
