package infraestructura

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// EspacioDeTrabajo es el espacio de trabajo de verdad, en disco: bajo raiz/<proyecto>/<pipeline>/<ambiente>/
// hay un directorio por paso, que se rehace entero al empezar cada intento (DEC-06.19) y otra vez cuando le toca
// a cada paso (RehacerElPaso). Cualquier otra cosa
// que los comandos del pipeline escriban en el ambiente, este puerto ni la lee ni la borra.
type EspacioDeTrabajo struct {
	raiz string
}

var _ dominio.EspacioDeTrabajo = (*EspacioDeTrabajo)(nil)

func NuevoEspacioDeTrabajo(raiz string) *EspacioDeTrabajo {
	return &EspacioDeTrabajo{raiz: raiz}
}

func (e *EspacioDeTrabajo) Ubicar(fuenteDelProyecto, fuenteDelPipeline, ambiente string) (dominio.Ubicacion, error) {
	proyecto, err := nombreDeLaFuente("FuenteDelProyecto", fuenteDelProyecto)
	if err != nil {
		return dominio.Ubicacion{}, fmt.Errorf("ejecución: el proyecto %q: %w", fuenteDelProyecto, err)
	}
	pipeline, err := nombreDeLaFuente("FuenteDelPipeline", fuenteDelPipeline)
	if err != nil {
		return dominio.Ubicacion{}, fmt.Errorf("ejecución: el pipeline %q: %w", fuenteDelPipeline, err)
	}
	return dominio.Ubicacion{Proyecto: proyecto, Pipeline: pipeline, Ambiente: ambiente}, nil
}

func (e *EspacioDeTrabajo) directorioDelAmbiente(u dominio.Ubicacion) string {
	return filepath.Join(e.raiz, u.Proyecto, u.Pipeline, u.Ambiente)
}

func (e *EspacioDeTrabajo) DirectorioDelPaso(u dominio.Ubicacion, paso string) string {
	return filepath.Join(e.directorioDelAmbiente(u), paso)
}

func (e *EspacioDeTrabajo) RehacerParteDelMotor(ctx context.Context, u dominio.Ubicacion, pasos []dominio.PasoDeEjecucion) error {
	for _, paso := range pasos {
		if err := e.RehacerElPaso(ctx, u, paso); err != nil {
			return err
		}
	}
	return nil
}

// RehacerElPaso borra el directorio del paso y lo vuelve a crear con su material declarado. No sigue enlaces
// simbólicos: si el directorio del ambiente es un enlace, no se toca nada y es un error (el paso correría en
// otro sitio); si lo es el del paso, se borra el enlace y no aquello a lo que apunta; y cada fichero se crea
// nuevo, nunca a través de un enlace que ya estuviera.
//
// Lo que queda fuera: los directorios de más arriba (proyecto, pipeline) y todo lo que los comandos comparten a
// propósito —el resto del ambiente, el material del proyecto—, que este puerto no rehace.
func (e *EspacioDeTrabajo) RehacerElPaso(_ context.Context, u dominio.Ubicacion, paso dominio.PasoDeEjecucion) error {
	if err := e.rehacerElPaso(u, paso); err != nil {
		return fmt.Errorf("ejecución: el ambiente %q: %w: %w", u.Ambiente, dominio.ErrNoDisponible, err)
	}
	return nil
}

func (e *EspacioDeTrabajo) rehacerElPaso(u dominio.Ubicacion, paso dominio.PasoDeEjecucion) error {
	directorioDelAmbiente := e.directorioDelAmbiente(u)
	if err := os.MkdirAll(directorioDelAmbiente, 0o755); err != nil {
		return err
	}
	if err := comprobarQueEsUnDirectorio(directorioDelAmbiente); err != nil {
		return err
	}
	directorioDelPaso := e.DirectorioDelPaso(u, paso.Nombre())
	if err := os.RemoveAll(directorioDelPaso); err != nil { // de un enlace, borra el enlace
		return err
	}
	if err := os.Mkdir(directorioDelPaso, 0o755); err != nil { // Mkdir y no MkdirAll: tiene que crearlo ahora
		return err
	}
	for _, f := range paso.Material {
		if err := escribirFichero(directorioDelPaso, f); err != nil {
			return err
		}
	}
	return nil
}

// comprobarQueEsUnDirectorio falla si la ruta es un enlace simbólico, o cualquier cosa que no sea un directorio.
func comprobarQueEsUnDirectorio(ruta string) error {
	info, err := os.Lstat(ruta)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s no es un directorio de verdad (%s): no se trabaja a través de un enlace", ruta, info.Mode().Type())
	}
	return nil
}

func escribirFichero(directorioDelPaso string, f dominio.FicheroDeclarado) error {
	destino := filepath.Join(directorioDelPaso, filepath.FromSlash(f.Ruta()))
	if err := os.MkdirAll(filepath.Dir(destino), 0o755); err != nil {
		return err
	}
	if f.Enlace() != "" {
		return os.Symlink(filepath.FromSlash(f.Enlace()), destino)
	}
	return escribirNuevo(destino, f.Contenido(), permisosDe(f))
}

func permisosDe(f dominio.FicheroDeclarado) os.FileMode {
	if f.Ejecutable() {
		return 0o755
	}
	return 0o644
}

// escribirNuevo crea el fichero, que no puede existir: con O_EXCL, si en su sitio hay un enlace simbólico la
// escritura falla en vez de seguirlo. os.WriteFile lo seguiría, y escribiría donde el enlace dijera.
func escribirNuevo(ruta, contenido string, permisos os.FileMode) error {
	fichero, err := os.OpenFile(ruta, os.O_WRONLY|os.O_CREATE|os.O_EXCL, permisos)
	if err != nil {
		return err
	}
	if _, err := fichero.WriteString(contenido); err != nil {
		_ = fichero.Close()
		return err
	}
	return fichero.Close()
}

// InterpolarPlantillas reescribe los ficheros que el paso marca como plantilla — llamarlo justo antes de
// ejecutar los comandos del paso es lo que permite que una plantilla use variables producidas por pasos
// anteriores (docs/modelo/contextos/ejecucion.md, «Espacio de trabajo»). Lo que interpola es el contenido
// declarado, no lo que haya en el disco, y lo escribe en un fichero nuevo: ni lee ni escribe a través de un
// enlace que alguien haya puesto en su sitio.
func (e *EspacioDeTrabajo) InterpolarPlantillas(
	_ context.Context, u dominio.Ubicacion, paso dominio.PasoDeEjecucion, interpolar dominio.Interpolador,
) error {
	directorioDelPaso := e.DirectorioDelPaso(u, paso.Nombre())
	for _, f := range paso.Material {
		if !f.Plantilla() {
			continue
		}
		interpolado, err := interpolar(f.Contenido())
		if err != nil {
			return fmt.Errorf("ejecución: interpolar la plantilla %q del paso %q: %w", f.Ruta(), paso.Nombre(), err)
		}
		ruta := filepath.Join(directorioDelPaso, filepath.FromSlash(f.Ruta()))
		if err := os.Remove(ruta); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("ejecución: reescribir la plantilla %q del paso %q: %w", f.Ruta(), paso.Nombre(), err)
		}
		if err := escribirNuevo(ruta, interpolado, permisosDe(f)); err != nil {
			return fmt.Errorf("ejecución: reescribir la plantilla %q del paso %q: %w", f.Ruta(), paso.Nombre(), err)
		}
	}
	return nil
}
