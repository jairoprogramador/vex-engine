// Package fingerprint contiene la primitiva de identidad del motor: la huella
// del contenido de un árbol de archivos.
//
// La regla canónica vive aquí, sin I/O, sobre el puerto TreeSource. El recorrido
// del sustrato —disco, memoria, un clon— vive en infraestructura. Esa partición
// es lo que permite que la regla se especifique y se verifique con vectores en
// memoria: ver SPEC-v1.md, que es normativo.
package fingerprint

import (
	"fmt"
	"strings"
)

// Version identifica la regla con la que se calculó una huella. Toda huella
// lleva el prefijo en su representación externa: sin él, corregir una
// divergencia documentada de la v1 sería indistinguible de un bug.
const Version = "v1"

// hashHexLen es la longitud en hexadecimal de un sha256.
const hashHexLen = 64

// separator separa la versión del hash en la representación canónica.
const separator = ":"

// Fingerprint es un value object: la identidad de un contenido bajo una versión
// concreta de la regla. Dos huellas son iguales si coinciden versión y hash; una
// huella v1 y una v2 del mismo árbol NO son iguales, y esa es la razón de ser
// del prefijo.
//
// El valor cero no es una identidad válida: representa «sin huella». Una huella
// vacía indistinguible de un error es exactamente lo que esta primitiva existe
// para erradicar, así que String() del valor cero devuelve la cadena vacía y
// IsZero() está para preguntarlo explícitamente.
type Fingerprint struct {
	version string
	hash    string
}

// New construye una huella de la versión actual a partir de un sha256 en
// hexadecimal minúsculo.
func New(hash string) (Fingerprint, error) {
	return newVersioned(Version, hash)
}

// Parse lee la representación canónica "<version>:<hash>".
func Parse(text string) (Fingerprint, error) {
	version, hash, found := strings.Cut(text, separator)
	if !found {
		return Fingerprint{}, fmt.Errorf(
			"fingerprint: %q no tiene la forma \"<version>:<hash>\"", text)
	}
	return newVersioned(version, hash)
}

func newVersioned(version, hash string) (Fingerprint, error) {
	if version == "" {
		return Fingerprint{}, fmt.Errorf("fingerprint: versión vacía")
	}
	if len(hash) != hashHexLen {
		return Fingerprint{}, fmt.Errorf(
			"fingerprint: el hash %q no tiene %d caracteres hexadecimales", hash, hashHexLen)
	}
	for i := 0; i < len(hash); i++ {
		c := hash[i]
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isHex {
			return Fingerprint{}, fmt.Errorf(
				"fingerprint: el hash %q no es hexadecimal minúsculo", hash)
		}
	}
	return Fingerprint{version: version, hash: hash}, nil
}

// String es la representación canónica y externa: "v1:<sha256 hex>".
// Es la forma en la que la huella se persiste, se transmite y se compara fuera
// del dominio.
func (f Fingerprint) String() string {
	if f.IsZero() {
		return ""
	}
	return f.version + separator + f.hash
}

// Version es la versión de la regla con la que se calculó.
func (f Fingerprint) Version() string { return f.version }

// Hash es el sha256 en hexadecimal, sin el prefijo de versión.
func (f Fingerprint) Hash() string { return f.hash }

// IsZero indica que no hay huella.
func (f Fingerprint) IsZero() bool { return f.version == "" || f.hash == "" }

// Equals es la regla de igualdad del value object.
func (f Fingerprint) Equals(other Fingerprint) bool {
	return f.version == other.version && f.hash == other.hash
}
