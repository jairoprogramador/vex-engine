package cache

import "context"

// Entries es el almacén de entradas de caché. Un solo puerto, un solo agregado.
//
// Sustituye a cuatro repositorios de ~110 líneas cada uno, idénticos salvo el
// nombre del archivo y el tipo del DTO, por dos implementaciones. Cuatro
// repositorios que hacen lo mismo con cuatro DTOs no son un exceso de
// responsabilidad sino su AUSENCIA: nadie era dueño de «el estado de
// re-ejecución». Ahora hay un dueño.
//
// El dominio lo define y la infraestructura lo implementa: hoy archivo, y desde
// la spec 21 el destino remoto. El contract test de la spec 02 se sostiene sobre
// este puerto único en vez de sobre ocho implementaciones que nadie comparaba.
type Entries interface {
	// Get consulta si existe entrada para una clave.
	//
	// Consultar NO crea la entrada. Es la idempotencia que la spec 09 §9.10
	// convirtió en el observable de que evaluar no muta el sistema, y aquí sigue
	// siendo la red de regresión: si `Get` escribiera, un proceso muerto entre
	// la consulta y el final del paso volvería a dejar grabado «ya se hizo» para
	// un paso que nunca terminó.
	Get(ctx *context.Context, key CacheKey) (Entry, bool, error)

	// Put escribe una entrada. SÓLO se llama desde el camino de éxito de un
	// paso, después de que su último comando terminó bien.
	Put(ctx *context.Context, key CacheKey, entry Entry) error
}
