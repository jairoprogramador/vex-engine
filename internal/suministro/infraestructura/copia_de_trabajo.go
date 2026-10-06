package infraestructura

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/jairoprogramador/vex-engine/internal/suministro/dominio"
)

// PonerCopiaDeTrabajo copia a un material lo que entraría en un commit si se añadiera todo: lo que sigue el
// índice, y lo que no sigue y no ignora ningún .gitignore de la copia. Así, una copia de trabajo sin cambios da
// el mismo hash que su commit, y lo que genera la tecnología (target/, node_modules/) no entra.
//
//   - Solo cuentan los .gitignore de la copia: .git/info/exclude y la configuración global dependen de la
//     máquina, y el mismo contenido daría otro material en otra.
//   - Un submódulo y un repositorio anidado no entran, igual que no entran al extraer un commit.
//   - Si el directorio no es la raíz de un repositorio, no hay índice, y solo cuentan los .gitignore.
//   - No se normalizan los fines de línea: con core.autocrlf, la copia no da el hash de su commit.
func (r *RepositoriosLocales) PonerCopiaDeTrabajo(ctx context.Context, copia dominio.CopiaDeTrabajo) (string, error) {
	raiz := copia.String()
	info, err := os.Stat(raiz)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
		return "", fmt.Errorf("%w: la copia de trabajo %s no es un directorio", dominio.ErrNoExiste, raiz)
	}
	if err != nil {
		return "", fmt.Errorf("repositorios locales: la copia de trabajo %s: %w", raiz, err)
	}
	seguidos, err := leerIndice(raiz)
	if err != nil {
		return "", err
	}
	return r.poner(func(material *escritura) error {
		c := copiador{ctx: ctx, raiz: raiz, seguidos: seguidos, material: material}
		return c.directorio(nil, nil)
	})
}

// seguimiento es lo que sigue el índice de una copia de trabajo. Las rutas van con '/'.
type seguimiento struct {
	ficheros    map[string]bool
	directorios map[string]bool // los que contienen, a cualquier profundidad, un fichero seguido
	submodulos  map[string]bool
}

func leerIndice(raiz string) (seguimiento, error) {
	s := seguimiento{ficheros: map[string]bool{}, directorios: map[string]bool{}, submodulos: map[string]bool{}}
	repo, err := git.PlainOpen(raiz)
	if errors.Is(err, git.ErrRepositoryNotExists) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("repositorios locales: abrir la copia de trabajo %s: %w", raiz, err)
	}
	indice, err := repo.Storer.Index()
	if err != nil {
		return s, fmt.Errorf("repositorios locales: el índice de %s: %w", raiz, err)
	}
	for _, entrada := range indice.Entries {
		if entrada.Mode == filemode.Submodule {
			s.submodulos[entrada.Name] = true
			continue
		}
		s.ficheros[entrada.Name] = true
		for dir := path.Dir(entrada.Name); dir != "."; dir = path.Dir(dir) {
			s.directorios[dir] = true
		}
	}
	return s, nil
}

type copiador struct {
	ctx      context.Context
	raiz     string
	seguidos seguimiento
	material *escritura
}

// directorio copia un directorio de la copia de trabajo con las reglas de los .gitignore de sus padres y las
// del suyo. Gana la última regla que casa, así que las de un directorio mandan sobre las de sus padres.
func (c *copiador) directorio(ruta []string, reglas []gitignore.Pattern) error {
	origen := filepath.Join(append([]string{c.raiz}, ruta...)...)
	propias, err := leerGitignore(filepath.Join(origen, ".gitignore"), ruta)
	if err != nil {
		return err
	}
	reglas = append(slices.Clip(reglas), propias...)
	ignorado := gitignore.NewMatcher(reglas)

	entradas, err := os.ReadDir(origen)
	if err != nil {
		return err
	}
	for _, entrada := range entradas {
		if err := c.ctx.Err(); err != nil {
			return err
		}
		if strings.EqualFold(entrada.Name(), ".git") {
			continue
		}
		hija := append(slices.Clip(ruta), entrada.Name())
		nombre := strings.Join(hija, "/")
		camino := filepath.Join(origen, entrada.Name())
		switch tipo := entrada.Type(); {
		case tipo.IsDir():
			if c.seguidos.submodulos[nombre] || esRepositorio(camino) {
				continue
			}
			if ignorado.Match(hija, true) && !c.seguidos.directorios[nombre] {
				continue
			}
			if err := c.directorio(hija, reglas); err != nil {
				return err
			}
		case tipo&fs.ModeSymlink != 0:
			if !c.seguidos.ficheros[nombre] && ignorado.Match(hija, false) {
				continue
			}
			destino, err := os.Readlink(camino)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			if err := c.material.enlace(nombre, destino); err != nil {
				return err
			}
		case tipo.IsRegular():
			if !c.seguidos.ficheros[nombre] && ignorado.Match(hija, false) {
				continue
			}
			if err := c.fichero(camino, nombre); err != nil {
				return err
			}
		}
		// Un socket o un dispositivo no es contenido.
	}
	return nil
}

// fichero copia un fichero con su bit de ejecución. Si desapareció mientras se recorría la copia, no está.
func (c *copiador) fichero(camino, nombre string) error {
	f, err := os.Open(camino)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	return c.material.fichero(nombre, info.Mode().Perm()&0o111 != 0, f)
}

// leerGitignore lee las reglas de un .gitignore como las lee go-git: una por cada línea que no está vacía ni
// empieza por #. Un .gitignore que no es un fichero regular no tiene reglas: seguir un enlace para leerlas las
// sacaría de la copia.
func leerGitignore(fichero string, ruta []string) ([]gitignore.Pattern, error) {
	info, err := os.Lstat(fichero)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil
	}
	contenido, err := os.ReadFile(fichero)
	if err != nil {
		return nil, err
	}
	var reglas []gitignore.Pattern
	for _, linea := range strings.Split(string(contenido), "\n") {
		linea = strings.TrimSuffix(linea, "\r")
		if strings.HasPrefix(linea, "#") || strings.TrimSpace(linea) == "" {
			continue
		}
		reglas = append(reglas, gitignore.ParsePattern(linea, ruta))
	}
	return reglas, nil
}

func esRepositorio(directorio string) bool {
	_, err := os.Lstat(filepath.Join(directorio, ".git"))
	return err == nil
}
