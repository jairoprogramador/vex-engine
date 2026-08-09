package deployment

// ContentIDVersion identifica la regla con la que se compone la identidad de un
// `Content`, y es INDEPENDIENTE de las tres versiones de huella (`v1`,
// `inst-v1`, `vars-v1`) y de la de la clave de caché (`ck-v1`).
//
// Son versionados distintos porque son reglas distintas sobre materiales
// distintos: un `content_id` puede saltar a v2 sin que ninguna huella se mueva,
// y una huella puede saltar a v2 sin que la regla de composición del objeto
// cambie ni un byte. Leer los dos prefijos como si fueran el mismo es el error
// que la spec 17 §8 cierra por escrito.
const ContentIDVersion = "cnt-v1"

// contentIDKind nombra el identificador en los mensajes de error.
const contentIDKind = "content_id"

// ContentID es la identidad de lo que se pretende ejecutar: el resumen de la
// forma canónica de un `Content`.
//
// Es OPACO a propósito —nadie fuera de este paquete lo construye campo a
// campo—, y por la misma razón que `cache.CacheKey`: un identificador
// parcialmente construido es la vía por la que una dimensión se cae del hash sin
// que nadie tenga que decidir excluirla. `Version` y `Hash` se exponen para que
// un almacén pueda derivar una ruta, no para recomponer la identidad.
//
// La diferencia con `CacheKey` es su ciclo de vida, y es la que agrava todo lo
// demás: una colisión en el caché cuesta un paso saltado; una colisión aquí
// cuesta dos despliegues distintos con la misma identidad, PARA SIEMPRE.
type ContentID struct {
	versionedHash
}

// newContentID compone la identidad a partir del hash ya calculado. No es
// exportada: la única forma legítima de obtener un `ContentID` es pedírselo a un
// `Content` construido y validado.
func newContentID(hash string) (ContentID, error) {
	value, err := newVersionedHash(contentIDKind, ContentIDVersion, hash)
	if err != nil {
		return ContentID{}, err
	}
	return ContentID{versionedHash: value}, nil
}

// ParseContentID lee la representación canónica "cnt-v1:<hash>". La necesitan el
// almacén de objetos y el linaje, que la leen de disco.
func ParseContentID(text string) (ContentID, error) {
	value, err := parseVersionedHash(contentIDKind, text)
	if err != nil {
		return ContentID{}, err
	}
	return ContentID{versionedHash: value}, nil
}

// Equals es la regla de igualdad del value object: dos identidades de versiones
// distintas NUNCA son iguales, aunque el hash coincida.
func (id ContentID) Equals(other ContentID) bool {
	return id.equals(other.versionedHash)
}
