package borde

import (
	"errors"
	"fmt"
	"slices"
)

// VersionesSoportadas son las versiones del lenguaje publicado que el borde entiende (DEC-05.6): cada petición
// dice la suya, y una que no está aquí se rechaza antes de tocar ningún contexto.
var VersionesSoportadas = []string{"1"}

// ErrVersionNoSoportada: la petición trae una versión del lenguaje publicado que este borde no entiende.
var ErrVersionNoSoportada = errors.New("borde: versión no soportada")

func comprobarVersion(version string) error {
	if slices.Contains(VersionesSoportadas, version) {
		return nil
	}
	return fmt.Errorf("%w: %q (soportadas: %v)", ErrVersionNoSoportada, version, VersionesSoportadas)
}
