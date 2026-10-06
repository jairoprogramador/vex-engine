package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// longitudDelNombre son los caracteres hexadecimales que se conservan del SHA-256: 64 bits, sin colisiones
// prácticas para los proyectos y pipelines de una máquina, y un nombre corto que cabe en cualquier ruta.
const longitudDelNombre = 16

// Ubicacion es dónde vive el espacio de trabajo de un ambiente: proyecto y pipeline ya convertidos en nombres
// de directorio (NombreDeDirectorio), y el ambiente tal cual. Un proyecto que usa varios pipelines, o un
// pipeline que usan varios proyectos, nunca comparten directorio.
type Ubicacion struct {
	Proyecto string
	Pipeline string
	Ambiente string
}

// NombreDeDirectorio convierte la referencia de una fuente —la URL de su remoto, o el nombre de su directorio
// si no tiene— en un nombre seguro como directorio en cualquier sistema operativo: los primeros caracteres
// hexadecimales del SHA-256 de su forma canónica. Es determinista: la misma referencia da siempre el mismo
// nombre, y variantes de la misma URL (https o ssh, con o sin ".git", host en mayúsculas) dan el mismo.
func NombreDeDirectorio(referencia string) (string, error) {
	canonica := canonicaDeLaReferencia(referencia)
	if canonica == "" {
		return "", invalido("la referencia de la fuente está vacía")
	}
	suma := sha256.Sum256([]byte(canonica))
	return hex.EncodeToString(suma[:])[:longitudDelNombre], nil
}

// canonicaDeLaReferencia lleva la referencia a "host/ruta" para una URL (con o sin esquema, https o scp), y la
// deja como está para lo que no lo es, el nombre de un directorio. Sin usuario, sin ".git" ni "/" finales, y con
// el host en minúsculas: la ruta conserva sus mayúsculas, porque los servidores las distinguen.
func canonicaDeLaReferencia(referencia string) string {
	r := strings.TrimSpace(referencia)
	if r == "" {
		return ""
	}
	host, ruta, esURL := partirURL(r)
	if !esURL {
		return r
	}
	ruta = strings.TrimSuffix(strings.TrimRight(ruta, "/"), ".git")
	ruta = strings.Trim(ruta, "/")
	return strings.ToLower(host) + "/" + ruta
}

func partirURL(r string) (host, ruta string, esURL bool) {
	if strings.Contains(r, "://") {
		u, err := url.Parse(r)
		if err != nil || u.Hostname() == "" {
			return "", "", false
		}
		return u.Hostname(), u.Path, true
	}
	// forma scp de git: usuario@host:ruta
	if _, resto, hayArroba := strings.Cut(r, "@"); hayArroba {
		if host, ruta, hayRuta := strings.Cut(resto, ":"); hayRuta && host != "" && !strings.Contains(host, "/") {
			return host, ruta, true
		}
	}
	return "", "", false
}
