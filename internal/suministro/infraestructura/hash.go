package infraestructura

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/suministro/dominio"
)

// HashDeContenido es el hash del contenido de un directorio, y es lo mínimo (IT-10 DEC-10.2): lo único que
// promete es cambiar si cambia el contenido. Qué entra es técnica (IT-02 DEC-02.18), y se escribe aquí para que
// cambiarlo sea una elección y no un descuido.
//
// Regla v1:
//
//   - Entran los ficheros y los enlaces, cada uno con su ruta relativa al directorio y '/' como separador. Los
//     directorios no entran: uno vacío es igual que uno que no está, como en un commit.
//   - Un fichero entra con su contenido y con si es ejecutable (cualquier bit de ejecución). Un enlace entra
//     con su destino como texto, sin seguirlo. No se normaliza nada más: ni los fines de línea ni el resto de
//     los permisos.
//   - Cada uno da la línea «<tipo> <sha256> <ruta>\x00», con el tipo f (fichero), x (ejecutable) o l (enlace) y
//     el SHA-256 en hexadecimal del contenido o del destino. Las líneas se ordenan por ruta, byte a byte.
//   - El hash es «contenido-v1:» seguido del SHA-256, en hexadecimal, de todas las líneas juntas.
//
// Si cambia lo que entra, cambia el prefijo: así un hash de una regla nunca es igual a uno de otra, y un cambio
// de regla no se hace pasar por un cambio del contenido.
type HashDeContenido struct{}

var _ dominio.Hashes = HashDeContenido{}

const prefijoHash = "contenido-v1:"

func (HashDeContenido) DeUnDirectorio(ctx context.Context, directorio string) (dominio.Hash, error) {
	type linea struct{ ruta, texto string }
	var lineas []linea
	err := filepath.WalkDir(directorio, func(camino string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relativa, err := filepath.Rel(directorio, camino)
		if err != nil {
			return err
		}
		ruta := filepath.ToSlash(relativa)
		tipo, suma, err := sumar(camino, d)
		if err != nil {
			return err
		}
		lineas = append(lineas, linea{ruta: ruta, texto: tipo + " " + suma + " " + ruta + "\x00"})
		return nil
	})
	if err != nil {
		return dominio.Hash{}, fmt.Errorf("hash del contenido de %s: %w", directorio, err)
	}

	slices.SortFunc(lineas, func(a, b linea) int { return strings.Compare(a.ruta, b.ruta) })
	total := sha256.New()
	for _, l := range lineas {
		io.WriteString(total, l.texto)
	}
	return dominio.NuevoHash(prefijoHash + hex.EncodeToString(total.Sum(nil)))
}

func sumar(camino string, d fs.DirEntry) (tipo, suma string, err error) {
	switch {
	case d.Type()&fs.ModeSymlink != 0:
		destino, err := os.Readlink(camino)
		if err != nil {
			return "", "", err
		}
		s := sha256.Sum256([]byte(destino))
		return "l", hex.EncodeToString(s[:]), nil
	case d.Type().IsRegular():
		f, err := os.Open(camino)
		if err != nil {
			return "", "", err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return "", "", err
		}
		s := sha256.New()
		if _, err := io.Copy(s, f); err != nil {
			return "", "", err
		}
		tipo = "f"
		if info.Mode().Perm()&0o111 != 0 {
			tipo = "x"
		}
		return tipo, hex.EncodeToString(s.Sum(nil)), nil
	}
	return "", "", fmt.Errorf("%s no es un fichero ni un enlace", camino)
}
