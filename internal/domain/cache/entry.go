package cache

import "time"

// DefaultTTL es cuánto vale una entrada antes de caducar.
//
// Son los 30 días que la regla de tiempo aplicaba SÓLO al paso `test`. Ahora
// aplican a todos, que es la otra cara de la pérdida de granularidad que la
// spec 10 §5.3 acepta a cambio de correctitud: la selección de comprobaciones
// por nombre de paso estaba cableada en el motor, y eso es justo lo que P1
// deroga. La spec 15 devuelve la granularidad declarada por el pipeline.
//
// El TTL NO es material de la clave: el tiempo no es propiedad del contenido.
// Volver a ejecutar por caducidad produce EXACTAMENTE la misma clave, y la
// entrada nueva sustituye a la vieja en su sitio.
const DefaultTTL = 30 * 24 * time.Hour

// Provenance es quién escribió una entrada y cuándo.
//
// Sin esto, «se salta porque ya está en caché» con una clave opaca deja sin
// respuesta la pregunta «¿cuándo se probó esto por última vez?» — que ES la
// afirmación de valor del motor. La spec 17 la amplía con `deployment_id` y
// `attempt`; aquí basta con que el hecho se guarde en vez de perderse.
type Provenance struct {
	ExecutionID string
	At          time.Time
}

// Entry es una entrada de caché: de SOLA PRESENCIA.
//
// No guarda resultado reutilizable. Guarda que *este contenido exacto ya se
// ejecutó con éxito aquí*, quién lo hizo y hasta cuándo vale. El contenido
// reutilizable —el almacén de variables— se queda donde está hasta la spec 11, y
// se queda a propósito: tiene reglas de borrado OPUESTAS. El caché se puede
// borrar entero sin consecuencias; un ARN guardado en el almacén es la pista de
// un recurso real y no se borra nunca.
type Entry struct {
	// ExpiresAt es nil cuando la entrada no caduca.
	ExpiresAt  *time.Time
	ProducedBy Provenance
}

// NewEntry construye la entrada que deja un paso que acaba de terminar bien.
//
// El instante lo pone el llamador y viene del puerto `shared.Clock` (spec 07):
// no hay `time.Now()` en el dominio, y por eso los dos lados del borde de los 30
// días se pueden probar sin esperar treinta días.
func NewEntry(producedBy Provenance, ttl time.Duration) Entry {
	if ttl <= 0 {
		return Entry{ProducedBy: producedBy}
	}
	expiresAt := producedBy.At.Add(ttl)
	return Entry{ExpiresAt: &expiresAt, ProducedBy: producedBy}
}

// IsExpired dice si la entrada ya no vale a fecha de `now`.
//
// El borde es exclusivo: una entrada expira CUANDO se alcanza su instante de
// expiración, no después. Es la misma frontera que comparaba la regla de tiempo.
func (e Entry) IsExpired(now time.Time) bool {
	if e.ExpiresAt == nil {
		return false
	}
	return !now.Before(*e.ExpiresAt)
}
