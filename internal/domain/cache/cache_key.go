package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// KeyVersion identifica la regla con la que se compone una clave. Toda clave lo
// lleva en su representación externa.
//
// Es la tercera capa de versionado y la que cierra el mecanismo: las tres
// huellas llevan la suya, y la clave se compone sobre sus formas canónicas
// COMPLETAS —con prefijo—, no sobre los hashes pelados. Consecuencia: un salto a
// `v2` de cualquiera de las tres reglas invalida todas las claves emitidas **sin
// una línea de código extra**. Es el OCP que la spec 08 §5.2' prometía, y sólo
// se conserva mientras la composición sea sobre `String()`.
const KeyVersion = "ck-v1"

const (
	// keyHeader encabeza el material de la clave. Existe para que una clave
	// nunca pueda coincidir con el hash de otra cosa que se le parezca.
	keyHeader = "vex-cache-key/" + KeyVersion

	keyLineSep  = "\n"
	keyFieldSep = ":"

	hashHexLen = 64
)

// CacheKey es la dirección de contenido de un paso: el resumen de TODO lo que
// determina su resultado.
//
// Es un value object OPACO a propósito. Nadie fuera de este paquete puede
// construirlo campo a campo, y ésa es la propiedad importante: un `CacheKey`
// parcialmente construido es la vía por la que el ambiente se volvería a caer de
// la clave, que es exactamente lo que pasó con las cuatro claves anteriores.
// `Version` y `Hash` se exponen para que un almacén pueda derivar una ruta, no
// para recomponer la clave: con ellos no se puede construir una.
type CacheKey struct {
	version string
	hash    string
}

// NewCacheKey compone la clave a partir del material completo.
//
// Devuelve error si el material está incompleto. Que sea un error y no una
// clave degradada es deliberado: aguas arriba, «no se pudo componer la clave»
// significa ejecutar el paso y NO escribir entrada, y ausencia de entrada
// implica ejecutar. Es la misma semántica que la guarda de cero reglas de la
// spec 05 —«sin evidencia ⇒ ejecutar»—, conservada por construcción en vez de
// por código.
func NewCacheKey(m Material) (CacheKey, error) {
	if err := m.Validate(); err != nil {
		return CacheKey{}, err
	}

	sum := sha256.Sum256([]byte(canonicalMaterial(m)))
	return CacheKey{version: KeyVersion, hash: hex.EncodeToString(sum[:])}, nil
}

// canonicalMaterial es la forma normativa del material (SPEC-v1.md §3 de este
// paquete). Ocho líneas en orden fijo, con los siete valores entrecomillados:
// como `strconv.Quote` escapa todo carácter de control, ningún valor puede
// simular un salto de línea y hacerse pasar por otro reparto de campos.
func canonicalMaterial(m Material) string {
	return strings.Join([]string{
		keyHeader,
		strconv.Quote(m.Subject),
		strconv.Quote(m.Pipeline),
		strconv.Quote(m.Scope),
		strconv.Quote(m.Step),
		strconv.Quote(m.Instructions.String()),
		strconv.Quote(m.Variables.String()),
		strconv.Quote(m.Code.String()),
	}, keyLineSep)
}

// ParseCacheKey lee la representación canónica "<versión>:<hash>". La necesita
// un almacén para reconstruir la clave que leyó de disco.
func ParseCacheKey(text string) (CacheKey, error) {
	version, hash, found := strings.Cut(text, keyFieldSep)
	if !found {
		return CacheKey{}, fmt.Errorf(
			"cache: %q no tiene la forma \"<versión>:<hash>\"", text)
	}
	if version == "" {
		return CacheKey{}, fmt.Errorf("cache: clave sin versión")
	}
	if len(hash) != hashHexLen {
		return CacheKey{}, fmt.Errorf(
			"cache: el hash %q no tiene %d caracteres hexadecimales", hash, hashHexLen)
	}
	for i := 0; i < len(hash); i++ {
		c := hash[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return CacheKey{}, fmt.Errorf(
				"cache: el hash %q no es hexadecimal minúsculo", hash)
		}
	}
	return CacheKey{version: version, hash: hash}, nil
}

// String es la representación canónica y externa: "ck-v1:<sha256 hex>".
func (k CacheKey) String() string {
	if k.IsZero() {
		return ""
	}
	return k.version + keyFieldSep + k.hash
}

// Version es la versión de la regla de composición.
func (k CacheKey) Version() string { return k.version }

// Hash es el sha256 en hexadecimal, sin el prefijo.
func (k CacheKey) Hash() string { return k.hash }

// IsZero indica que no hay clave.
func (k CacheKey) IsZero() bool { return k.version == "" || k.hash == "" }

// Equals es la regla de igualdad del value object: dos claves de versiones
// distintas NUNCA son iguales, aunque el hash coincida.
func (k CacheKey) Equals(other CacheKey) bool {
	return k.version == other.version && k.hash == other.hash
}
