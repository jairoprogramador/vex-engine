package infraestructura

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/jairoprogramador/vex-engine/internal/historial/dominio"
)

const (
	extensionDeRegistro = ".json"
	digitosDePosicion   = 12
)

// AlmacenLocal guarda cada secuencia en un directorio y cada registro en un fichero con el número de su
// posición: <raíz>/<familia>/<nombre>/000000000001.json.
//
// No sobrescribir y escribir de forma condicional son la misma operación: el registro se escribe entero en
// un temporal y se publica con un enlace duro, que falla si la posición ya existe. Un lector nunca ve un
// registro a medias, y de dos escritores en la misma posición gana uno. Por eso necesita un sistema de
// ficheros con enlaces duros.
//
// Solo está disponible en la máquina donde está su raíz. Llevarlo a otras es de lo que se compre (RD-11).
type AlmacenLocal struct {
	raiz string
}

var _ Almacen = (*AlmacenLocal)(nil)

// NuevoAlmacenLocal usa un directorio que tiene que existir: si no se alcanza, no se escribe nada.
func NuevoAlmacenLocal(raiz string) (*AlmacenLocal, error) {
	info, err := os.Stat(raiz)
	if err != nil {
		return nil, fmt.Errorf("almacén local: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("almacén local: %s no es un directorio", raiz)
	}
	return &AlmacenLocal{raiz: raiz}, nil
}

func (a *AlmacenLocal) Leer(ctx context.Context, s Secuencia) ([][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := a.directorio(s)
	if err != nil {
		return nil, err
	}
	cantidad, err := contarPosiciones(dir)
	if err != nil {
		return nil, fmt.Errorf("almacén local: secuencia %s: %w", s, err)
	}
	registros := make([][]byte, 0, cantidad)
	for posicion := 1; posicion <= cantidad; posicion++ {
		datos, err := os.ReadFile(archivoDePosicion(dir, posicion))
		if err != nil {
			return nil, fmt.Errorf("almacén local: secuencia %s: %w", s, err)
		}
		registros = append(registros, datos)
	}
	return registros, nil
}

func (a *AlmacenLocal) Anadir(ctx context.Context, s Secuencia, posicion int, registro []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if posicion < 1 {
		return fmt.Errorf("almacén local: secuencia %s: la posición empieza en 1, no en %d", s, posicion)
	}
	dir, err := a.directorio(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("almacén local: secuencia %s: %w", s, err)
	}
	if posicion > 1 {
		if _, err := os.Stat(archivoDePosicion(dir, posicion-1)); err != nil {
			return fmt.Errorf("almacén local: secuencia %s: no se añade la posición %d sin la anterior: %w",
				s, posicion, err)
		}
	}

	temporal, err := escribirTemporal(dir, registro)
	if err != nil {
		return fmt.Errorf("almacén local: secuencia %s: %w", s, err)
	}
	defer os.Remove(temporal)

	if err := os.Link(temporal, archivoDePosicion(dir, posicion)); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("almacén local: secuencia %s, posición %d: %w", s, posicion, dominio.ErrConflicto)
		}
		return fmt.Errorf("almacén local: secuencia %s, posición %d: %w", s, posicion, err)
	}
	if err := sincronizarDirectorio(dir); err != nil {
		return fmt.Errorf("almacén local: secuencia %s: %w", s, err)
	}
	return nil
}

func (a *AlmacenLocal) Nombres(ctx context.Context, familia string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, err := a.directorio(Secuencia{Familia: familia})
	if err != nil {
		return nil, err
	}
	entradas, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("almacén local: familia %s: %w", familia, err)
	}
	var nombres []string
	for _, e := range entradas {
		if !e.IsDir() {
			continue
		}
		nombre, err := nombreDeSegmento(e.Name())
		if err != nil {
			return nil, err
		}
		nombres = append(nombres, nombre)
	}
	sort.Strings(nombres)
	return nombres, nil
}

func (a *AlmacenLocal) directorio(s Secuencia) (string, error) {
	familia, err := segmento(s.Familia)
	if err != nil {
		return "", err
	}
	if s.Nombre == "" {
		return filepath.Join(a.raiz, familia), nil
	}
	nombre, err := segmento(s.Nombre)
	if err != nil {
		return "", err
	}
	return filepath.Join(a.raiz, familia, nombre), nil
}

func archivoDePosicion(dir string, posicion int) string {
	return filepath.Join(dir, fmt.Sprintf("%0*d%s", digitosDePosicion, posicion, extensionDeRegistro))
}

// contarPosiciones cuenta los registros de un directorio y comprueba que van de 1 a n sin huecos. Ignora los
// temporales, que empiezan por punto, y los subdirectorios.
func contarPosiciones(dir string) (int, error) {
	entradas, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var posiciones []int
	for _, e := range entradas {
		nombre := e.Name()
		if e.IsDir() || strings.HasPrefix(nombre, ".") {
			continue
		}
		numero := strings.TrimSuffix(nombre, extensionDeRegistro)
		posicion, err := strconv.Atoi(numero)
		if len(numero) != digitosDePosicion || numero == nombre || err != nil {
			return 0, fmt.Errorf("fichero ajeno a la secuencia: %s", nombre)
		}
		posiciones = append(posiciones, posicion)
	}
	sort.Ints(posiciones)
	for k, posicion := range posiciones {
		if posicion != k+1 {
			return 0, fmt.Errorf("falta la posición %d", k+1)
		}
	}
	return len(posiciones), nil
}

func escribirTemporal(dir string, registro []byte) (string, error) {
	f, err := os.CreateTemp(dir, ".registro-*")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(registro); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// sincronizarDirectorio hace durable el enlace nuevo. Windows no permite sincronizar un directorio.
func sincronizarDirectorio(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return err
	}
	return d.Close()
}
