package infraestructura

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// nombreDeLaFuente da el nombre de directorio de una fuente: el de la URL de su remoto «origin» si es un
// repositorio que lo tiene, y si no, el del último tramo de su ruta. Nunca devuelve un nombre vacío.
func nombreDeLaFuente(fuente string) (string, error) {
	if fuente == "" {
		return "", fmt.Errorf("%w: la fuente está vacía", dominio.ErrInvalido)
	}
	if url := urlDelOrigen(fuente); url != "" {
		return dominio.NombreDeDirectorio(url)
	}
	return dominio.NombreDeDirectorio(ultimoTramo(fuente))
}

func urlDelOrigen(directorio string) string {
	repo, err := git.PlainOpen(directorio)
	if err != nil {
		return ""
	}
	origen, err := repo.Remote(git.DefaultRemoteName)
	if err != nil {
		return ""
	}
	if urls := origen.Config().URLs; len(urls) > 0 {
		return urls[0]
	}
	return ""
}

// ultimoTramo es el nombre del directorio de la fuente. Una ruta relativa como "." o ".." no tiene nombre, así
// que se resuelve antes; una fuente que ya es una URL sin remoto que abrir conserva su forma tal cual.
func ultimoTramo(fuente string) string {
	limpia := filepath.Clean(fuente)
	if limpia == "." || limpia == ".." {
		if absoluta, err := filepath.Abs(limpia); err == nil {
			limpia = absoluta
		}
	}
	if base := filepath.Base(limpia); base != "." && base != string(filepath.Separator) {
		return base
	}
	return limpia
}
