package infraestructura

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// nombreDelDirectorioEfimero es el del directorio efímero del paso. Es fijo, y lo único que este puerto borra:
// quien configura el motor dice dónde va, no qué se borra.
const nombreDelDirectorioEfimero = "vex-step-tmp"

// RutaDelDirectorioEfimero es dónde va el directorio efímero del paso: <base>/vex-step-tmp, con la base que da
// quien configura el motor. Tiene que ser un sitio del contenedor y no un volumen compartido: dos intentos a la
// vez sobre la misma base se lo pisarían.
//
// Sin base va al directorio temporal del sistema, con el número del proceso en el nombre, para que dos motores
// en la misma máquina no compartan el suyo. Esa ruta solo la conocen los comandos, por su variable de entorno.
func RutaDelDirectorioEfimero(base string) string {
	if base == "" {
		return filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", nombreDelDirectorioEfimero, os.Getpid()))
	}
	return filepath.Join(base, nombreDelDirectorioEfimero)
}

// DirectorioEfimero es el directorio efímero del paso, en disco y siempre en la misma ruta.
type DirectorioEfimero struct {
	ruta string
}

var _ dominio.DirectorioEfimero = DirectorioEfimero{}

func NuevoDirectorioEfimero(ruta string) DirectorioEfimero { return DirectorioEfimero{ruta: ruta} }

// Crear borra lo que haya en la ruta —si es un enlace simbólico, el enlace y no aquello a lo que apunta— y crea
// el directorio, vacío y solo para quien corre el motor (0700). Mkdir y no MkdirAll: tiene que crearlo ahora, no
// aceptar uno que alguien pusiera entre el borrado y la creación.
func (d DirectorioEfimero) Crear(context.Context) (string, error) {
	if err := os.MkdirAll(filepath.Dir(d.ruta), 0o755); err != nil {
		return "", fmt.Errorf("ejecución: el sitio del directorio efímero %s: %w", d.ruta, err)
	}
	if err := os.RemoveAll(d.ruta); err != nil {
		return "", fmt.Errorf("ejecución: vaciar el directorio efímero %s: %w", d.ruta, err)
	}
	if err := os.Mkdir(d.ruta, 0o700); err != nil {
		return "", fmt.Errorf("ejecución: crear el directorio efímero %s: %w", d.ruta, err)
	}
	// La máscara del proceso puede quitar permisos, pero no darlos; con Chmod quedan los que son, sea cual sea.
	if err := os.Chmod(d.ruta, 0o700); err != nil {
		return "", fmt.Errorf("ejecución: cerrar el directorio efímero %s: %w", d.ruta, err)
	}
	return d.ruta, nil
}

// Borrar quita el directorio con todo lo que tenga. Que no exista no es un error.
func (d DirectorioEfimero) Borrar(context.Context) error {
	if err := os.RemoveAll(d.ruta); err != nil {
		return fmt.Errorf("ejecución: borrar el directorio efímero %s: %w", d.ruta, err)
	}
	return nil
}
