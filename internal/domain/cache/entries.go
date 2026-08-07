package cache

import "context"

// Entries es el índice de contenido → registro. Un solo puerto, un solo
// agregado.
//
// **No participa en la decisión de re-ejecutar**, y ésa es su definición, no una
// nota (spec 11 §5.5). La decisión lee el último registro de la clave de estado;
// esto responde otra pregunta —«¿este contenido ya corrió ALGUNA vez, y cuál fue
// el registro?»— cuyos consumidores son de consulta: el `evidence_from` de la
// spec 17, el diagnóstico de la spec 22 y un futuro `vex plan`.
//
// De ahí salen sus dos propiedades, que no son las del almacén de registros:
//
//   - es DERIVABLE — se reconstruye recorriendo los registros;
//   - es DESECHABLE de verdad, no «desechable si nadie lo necesita».
//
// Y de ahí sale también su política de error: ilegible ⇒ ausente. Aquí la duda
// se resuelve sin arriesgar nada; en el almacén de registros, no (spec 11 §5.6).
type Entries interface {
	// Get consulta a qué registro apunta un contenido.
	//
	// Consultar NO crea la entrada. Es la idempotencia que la spec 09 §9.10
	// convirtió en observable de que evaluar no muta el sistema.
	Get(ctx *context.Context, key CacheKey) (Entry, bool, error)

	// Put escribe una entrada. SÓLO se llama desde el camino de éxito de un
	// step, después de que su registro quedó escrito: el índice apunta a
	// registros que existen.
	Put(ctx *context.Context, key CacheKey, entry Entry) error
}
