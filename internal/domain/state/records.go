package state

import "context"

// Records es el almacén de registros de step.
//
// **No tiene operación de borrado ni de actualización, y eso es el tipo
// diciendo la regla**: que la interfaz no ofrezca `Delete` es más fuerte que
// documentar que no se debe usar. La limpieza a mano de registros viejos es de
// la spec 22 (`vexd record gc`), y va por otra puerta a propósito.
//
// Sustituye a `step.VarsStoreRepository`, que ofrecía `Save` sobre una posición
// —un archivo por (pipeline, scope, step)— y por tanto sobrescribía por
// construcción.
type Records interface {
	// Append añade un registro bajo una clave. NUNCA sustituye a otro: dos
	// llamadas con la misma clave dejan DOS registros.
	//
	// Se llama sólo desde el camino de éxito de un step, después de que su
	// último comando terminó bien. Un step que empieza y no termina —incluidos
	// SIGKILL, OOM o un corte de luz— no deja nada, y la corrida siguiente lo
	// vuelve a ejecutar (spec 09 §5.2).
	Append(ctx *context.Context, key Key, record StepRecord) error

	// Last devuelve el ÚLTIMO registro de una clave, que es el que la decisión
	// de re-ejecutar consulta.
	//
	// «Último» es el mayor `record_id`: el orden lexicográfico de los ULID es su
	// orden temporal, así que obtenerlo no exige leer los demás.
	//
	// Ausencia NO es error —es la respuesta «este step nunca corrió aquí», y
	// aguas arriba significa ejecutar—. Un registro ILEGIBLE sí lo es: en el
	// índice, ilegible ⇒ ausente (fail-open, ante la duda ejecutar); aquí,
	// ilegible ⇒ error, porque la duda no se puede resolver ejecutando sin
	// arriesgar un recurso duplicado (spec 11 §5.6).
	Last(ctx *context.Context, key Key) (StepRecord, bool, error)
}
