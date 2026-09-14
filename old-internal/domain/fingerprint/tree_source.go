package fingerprint

import (
	"io"
	"io/fs"
)

// WalkFunc se invoca una vez por entrada del árbol.
//
//   - path: ruta relativa a la raíz, con "/" como separador, sin "./" delante.
//   - isDir: la entrada es un directorio. Un symlink NUNCA es un directorio,
//     apunte a donde apunte.
//   - isSymlink: la entrada es un enlace simbólico. No se sigue.
//   - mode: los permisos de la entrada. La regla v1 sólo mira el bit 0111.
//
// Devolver fs.SkipDir sobre un directorio omite su contenido; sobre cualquier
// otra entrada omite el resto del directorio que la contiene — la misma
// semántica que filepath.WalkDir. Cualquier otro error aborta el recorrido.
type WalkFunc func(path string, isDir bool, isSymlink bool, mode fs.FileMode) error

// TreeSource es el puerto por el que la regla ve un árbol de archivos. Lo define
// el dominio que lo consume; sus implementaciones —disco, memoria— viven en
// infraestructura.
//
// fs.FS no sirve como puerto: no expresa symlinks sin seguirlos, y desde la v1
// el destino de un symlink es material de identidad.
//
// El contrato que una implementación debe cumplir para que la huella sea
// reproducible (SPEC-v1.md §2):
//
//  1. Walk no emite la raíz, sólo sus descendientes.
//  2. Dentro de un directorio, las entradas se emiten en orden lexicográfico
//     ascendente por nombre, y un directorio se emite antes que su contenido.
//  3. Los symlinks no se siguen jamás, ni al recorrer ni al abrir.
//  4. Open devuelve el contenido de un archivo regular, o el destino de un
//     symlink —en forma de ruta con "/"— sin resolverlo.
//  5. Open devuelve un error que envuelve fs.ErrNotExist si la ruta no existe.
//
// El punto 2 importa más de lo que parece: la precedencia de reglas .gitignore
// de la v1 depende del orden del recorrido (SPEC-v1.md §5.4, divergencia (a)).
type TreeSource interface {
	Walk(fn WalkFunc) error
	Open(path string) (io.ReadCloser, error)
}
