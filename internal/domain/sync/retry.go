package sync

import (
	"context"
	"fmt"
	"time"
)

// Los valores por defecto del reintento. Son ACOTADOS a propósito y pequeños:
// con empuje por step hay como mucho 4–5 llamadas por ejecución, así que lo que
// se compra reintentando es sobrevivir a un parpadeo, no a una caída. Una caída
// la recoge el step siguiente, que reenvía desde el mismo `ack`.
//
// Por lo mismo se descartó un Circuit Breaker (§5.3'): con cinco muestras por
// ejecución no tendría con qué decidir nada.
const (
	defaultRetryAttempts = 3
	defaultRetryBase     = 200 * time.Millisecond
)

// RetryPolicy es cuántas veces se intenta un empuje y cuánto se espera entre
// intentos.
//
// La espera DOBLA en cada intento —200 ms, 400 ms— y está acotada por el número
// de intentos, no por un tiempo máximo: el pipeline sigue corriendo detrás y lo
// que no puede pasar es que el registro lo retenga. Un fallo agota los
// reintentos, emite `sync_failed` y devuelve el control.
type RetryPolicy struct {
	attempts int
	base     time.Duration
}

// DefaultRetryPolicy es la política del motor. Los números concretos son del
// CABLEADO y no del dominio: quien construya el motor puede darle otros sin que
// nada de aquí cambie.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{attempts: defaultRetryAttempts, base: defaultRetryBase}
}

// NewRetryPolicy compone una política. Menos de un intento no es una política
// más laxa: es no empujar, y eso se expresa no cableando el sincronizador.
func NewRetryPolicy(attempts int, base time.Duration) (RetryPolicy, error) {
	if attempts < 1 {
		return RetryPolicy{}, fmt.Errorf("sync: %d intentos no es una política de reintento", attempts)
	}
	if base < 0 {
		return RetryPolicy{}, fmt.Errorf("sync: la espera entre intentos no puede ser negativa")
	}
	return RetryPolicy{attempts: attempts, base: base}, nil
}

// Attempts son los intentos totales, incluido el primero.
func (p RetryPolicy) Attempts() int {
	if p.attempts < 1 {
		return defaultRetryAttempts
	}
	return p.attempts
}

// WaitBefore es lo que se espera ANTES del intento `attempt` (1 es el primero).
// Antes del primero no se espera: un destino sano no paga la latencia del que
// no lo está.
func (p RetryPolicy) WaitBefore(attempt int) time.Duration {
	if attempt <= 1 || p.base <= 0 {
		return 0
	}
	return p.base << (attempt - 2)
}

// Sleeper es la espera, como puerto.
//
// Por la misma razón que el reloj (spec 07 §5.1): dormir es una dependencia
// externa, y un `time.Sleep` dentro del dominio haría que la prueba de una
// política de reintento tardara lo que la política dice. Respeta la cancelación
// —devuelve el error del contexto— porque un `Ctrl-C` durante un backoff no
// puede quedarse esperando.
type Sleeper interface {
	Sleep(ctx *context.Context, duration time.Duration) error
}
