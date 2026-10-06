package infraestructura

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// escritura llena un material. Nada de lo que escribe sale de su raíz ni pasa por un enlace que haya puesto
// antes: en un sistema de ficheros que no distingue mayúsculas, un enlace «A» y un fichero «a/b» son el mismo
// camino. Tampoco escribe un .git: el material no es un repositorio.
type escritura struct {
	raiz string
}

func (e *escritura) fichero(ruta string, ejecutable bool, contenido io.Reader) error {
	destino, err := e.destino(ruta)
	if err != nil {
		return err
	}
	permisos := os.FileMode(0o644)
	if ejecutable {
		permisos = 0o755
	}
	f, err := os.OpenFile(destino, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permisos)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, contenido)
	if err == nil {
		// La umask no afecta a Chmod, y el bit de ejecución entra en el hash.
		err = f.Chmod(permisos)
	}
	return errors.Join(err, f.Close())
}

func (e *escritura) enlace(ruta, destinoDelEnlace string) error {
	destino, err := e.destino(ruta)
	if err != nil {
		return err
	}
	return os.Symlink(destinoDelEnlace, destino)
}

// destino es dónde se escribe una ruta del material, que va con '/'. Crea los directorios que faltan.
func (e *escritura) destino(ruta string) (string, error) {
	local := filepath.Clean(filepath.FromSlash(ruta))
	if !filepath.IsLocal(local) {
		return "", fmt.Errorf("la ruta %q sale del material", ruta)
	}
	partes := strings.Split(local, string(filepath.Separator))
	actual := e.raiz
	for i, parte := range partes {
		if strings.EqualFold(parte, ".git") {
			return "", fmt.Errorf("la ruta %q pasa por un .git, y el material no es un repositorio", ruta)
		}
		actual = filepath.Join(actual, parte)
		if i == len(partes)-1 {
			break
		}
		info, err := os.Lstat(actual)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := os.Mkdir(actual, 0o755); err != nil {
				return "", err
			}
		case err != nil:
			return "", err
		case !info.IsDir():
			return "", fmt.Errorf("la ruta %q pasa por %s, que no es un directorio", ruta, actual)
		}
	}
	return actual, nil
}
