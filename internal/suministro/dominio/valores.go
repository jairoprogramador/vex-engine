package dominio

import (
	"errors"
	"strings"
)

// Suministro no cambia nada, solo trae: no tiene entidades ni agregados, solo estos value objects.

// Fuente es de dónde viene el material: el repositorio del producto o el del pipeline. Qué es su ubicación lo
// sabe el acceso a los repositorios; para el acceso local, es un directorio.
type Fuente struct{ ubicacion string }

func NuevaFuente(ubicacion string) (Fuente, error) {
	if ubicacion == "" {
		return Fuente{}, invalido("la fuente no dice dónde está")
	}
	return Fuente{ubicacion: ubicacion}, nil
}

func (f Fuente) String() string { return f.ubicacion }

// Commit es un punto de la historia de una fuente con el que se vuelve a tener delante el material tal como
// era. Se guarda para volver atrás, no para saber quién cambió qué (IT-07 DEC-07.7). Es el identificador
// completo, en hexadecimal, SHA-1 o SHA-256: una abreviatura o una rama no son un punto fijo.
type Commit struct{ id string }

func NuevoCommit(id string) (Commit, error) {
	normalizado := strings.ToLower(id)
	if len(normalizado) != 40 && len(normalizado) != 64 {
		return Commit{}, invalido("%q no es un commit: se pide su identificador completo", id)
	}
	for _, c := range normalizado {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return Commit{}, invalido("%q no es un commit: no es hexadecimal", id)
		}
	}
	return Commit{id: normalizado}, nil
}

func (c Commit) String() string { return c.id }

// Hash responde a una sola pregunta: ¿cambió el contenido? Es el del código o el del pipeline, según la fuente.
// No es un commit: si alguien revierte un cambio, el commit es nuevo y el hash vuelve a ser el de antes.
type Hash struct{ valor string }

func NuevoHash(valor string) (Hash, error) {
	if valor == "" {
		return Hash{}, invalido("un hash no puede estar vacío")
	}
	return Hash{valor: valor}, nil
}

func (h Hash) String() string { return h.valor }

// CopiaDeTrabajo es el directorio donde alguien está trabajando, con cambios sin commit (IT-10 DEC-10.6).
type CopiaDeTrabajo struct{ directorio string }

func NuevaCopiaDeTrabajo(directorio string) (CopiaDeTrabajo, error) {
	if directorio == "" {
		return CopiaDeTrabajo{}, invalido("la copia de trabajo no dice dónde está")
	}
	return CopiaDeTrabajo{directorio: directorio}, nil
}

func (c CopiaDeTrabajo) String() string { return c.directorio }

// Material es lo que se pone delante: un directorio que no cambia para quien lo pidió, con el hash de su
// contenido y, si lo hay, su commit. Una copia de trabajo tiene hash, pero no commit (IT-10 DEC-10.6).
type Material struct {
	directorio string
	hash       Hash
	commit     Commit
}

func MaterialDeUnCommit(directorio string, hash Hash, commit Commit) (Material, error) {
	if commit == (Commit{}) {
		return Material{}, errors.New("suministro: el material de un commit dice cuál es su commit")
	}
	return nuevoMaterial(directorio, hash, commit)
}

func MaterialDeUnaCopiaDeTrabajo(directorio string, hash Hash) (Material, error) {
	return nuevoMaterial(directorio, hash, Commit{})
}

func nuevoMaterial(directorio string, hash Hash, commit Commit) (Material, error) {
	if directorio == "" {
		return Material{}, errors.New("suministro: el material no dice dónde está")
	}
	if hash == (Hash{}) {
		return Material{}, errors.New("suministro: el material no tiene hash")
	}
	return Material{directorio: directorio, hash: hash, commit: commit}, nil
}

func (m Material) Directorio() string { return m.directorio }

func (m Material) Hash() Hash { return m.hash }

// Commit devuelve falso si el material es una copia de trabajo.
func (m Material) Commit() (Commit, bool) { return m.commit, m.commit != (Commit{}) }
