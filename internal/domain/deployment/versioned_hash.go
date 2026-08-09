package deployment

import (
	"fmt"
	"strings"
)

const (
	// hashHexLen es la longitud en hexadecimal de un sha256.
	hashHexLen = 64

	// idSeparator separa la versión del hash en la representación canónica.
	idSeparator = ":"
)

// versionedHash es la DISCIPLINA de la spec 08 copiada, no su tipo.
//
// La decisión está escrita en la spec 17 §8 y conviene no revertirla por
// parecido: **`ContentID` NO reutiliza `fingerprint.Fingerprint`**. Son otra
// regla, sobre otro material, con su propio ciclo de versionado — un
// `content_id` puede saltar a v2 sin que ninguna huella se mueva, y al revés—.
// Lo que sí se copia es lo que la 08 dejó probado: constructor que valida la
// forma, `Parse` para la ida y vuelta, igualdad que incluye la versión, y valor
// cero que es explícitamente «sin identidad» en vez de una cadena vacía
// indistinguible de un error.
//
// Vive aquí, sin exportar, para que `ContentID` y `DeploymentID` compartan la
// validación sin compartir tipo: dos identidades distintas que el compilador no
// deja confundir, que es exactamente lo que la regla de independencia pide.
type versionedHash struct {
	version string
	hash    string
}

// newVersionedHash valida la forma: versión no vacía y 64 hexadecimales
// minúsculos. `kind` nombra el identificador en el mensaje de error, porque un
// «hash inválido» a secas no dice cuál de las dos identidades se estaba
// componiendo.
func newVersionedHash(kind, version, hash string) (versionedHash, error) {
	if version == "" {
		return versionedHash{}, fmt.Errorf("deployment: %s sin versión", kind)
	}
	if len(hash) != hashHexLen {
		return versionedHash{}, fmt.Errorf(
			"deployment: el hash %q de %s no tiene %d caracteres hexadecimales",
			hash, kind, hashHexLen)
	}
	for i := 0; i < len(hash); i++ {
		c := hash[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return versionedHash{}, fmt.Errorf(
				"deployment: el hash %q de %s no es hexadecimal minúsculo", hash, kind)
		}
	}
	return versionedHash{version: version, hash: hash}, nil
}

// parseVersionedHash lee la representación canónica "<versión>:<hash>".
func parseVersionedHash(kind, text string) (versionedHash, error) {
	version, hash, found := strings.Cut(text, idSeparator)
	if !found {
		return versionedHash{}, fmt.Errorf(
			"deployment: %q no tiene la forma \"<versión>:<hash>\" de %s", text, kind)
	}
	return newVersionedHash(kind, version, hash)
}

// String es la representación canónica y externa: "<versión>:<sha256 hex>".
//
// Es la forma en la que la identidad se persiste, se transmite y —lo que más
// pesa— se COMPONE: quien la use como material de otro identificador debe
// componer sobre esta cadena entera, nunca sobre el hash pelado. De ahí sale que
// un salto de versión invalide lo derivado sin una línea de código extra.
func (v versionedHash) String() string {
	if v.IsZero() {
		return ""
	}
	return v.version + idSeparator + v.hash
}

// Version es la versión de la regla con la que se compuso.
func (v versionedHash) Version() string { return v.version }

// Hash es el sha256 en hexadecimal, sin el prefijo.
func (v versionedHash) Hash() string { return v.hash }

// IsZero indica que no hay identidad.
func (v versionedHash) IsZero() bool { return v.version == "" || v.hash == "" }

func (v versionedHash) equals(other versionedHash) bool {
	return v.version == other.version && v.hash == other.hash
}
