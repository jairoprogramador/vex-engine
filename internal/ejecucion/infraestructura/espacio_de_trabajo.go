package infraestructura

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// EspacioDeTrabajo es el espacio de trabajo de verdad, en disco: bajo raiz/<proyecto>/<pipeline>/<ambiente>/
// hay un directorio por paso, que se rehace entero al empezar cada intento (DEC-06.19). Cualquier otra cosa
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

func (e *EspacioDeTrabajo) RehacerParteDelMotor(_ context.Context, u dominio.Ubicacion, pasos []dominio.PasoDeEjecucion) error {
	for _, paso := range pasos {
		directorioDelPaso := e.DirectorioDelPaso(u, paso.Nombre())
		if err := os.RemoveAll(directorioDelPaso); err != nil {
			return fmt.Errorf("ejecución: el ambiente %q: %w: %w", u.Ambiente, dominio.ErrNoDisponible, err)
		}
		if err := os.MkdirAll(directorioDelPaso, 0o755); err != nil {
			return fmt.Errorf("ejecución: el ambiente %q: %w: %w", u.Ambiente, dominio.ErrNoDisponible, err)
		}
		for _, f := range paso.Material {
			if err := escribirFichero(directorioDelPaso, f); err != nil {
				return fmt.Errorf("ejecución: el ambiente %q: %w: %w", u.Ambiente, dominio.ErrNoDisponible, err)
			}
		}
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
	permisos := os.FileMode(0o644)
	if f.Ejecutable() {
		permisos = 0o755
	}
	return os.WriteFile(destino, []byte(f.Contenido()), permisos)
}

// InterpolarPlantillas reescribe, en su sitio, los ficheros que el paso marca como plantilla — llamarlo justo
// antes de ejecutar los comandos del paso es lo que permite que una plantilla use variables producidas por
// pasos anteriores (docs/modelo/contextos/ejecucion.md, «Espacio de trabajo»).
func (e *EspacioDeTrabajo) InterpolarPlantillas(
	_ context.Context, u dominio.Ubicacion, paso dominio.PasoDeEjecucion, interpolar dominio.Interpolador,
) error {
	directorioDelPaso := e.DirectorioDelPaso(u, paso.Nombre())
	for _, f := range paso.Material {
		if !f.Plantilla() {
			continue
		}
		ruta := filepath.Join(directorioDelPaso, filepath.FromSlash(f.Ruta()))
		contenido, err := os.ReadFile(ruta)
		if err != nil {
			return fmt.Errorf("ejecución: leer la plantilla %q del paso %q: %w", f.Ruta(), paso.Nombre(), err)
		}
		interpolado, err := interpolar(string(contenido))
		if err != nil {
			return fmt.Errorf("ejecución: interpolar la plantilla %q del paso %q: %w", f.Ruta(), paso.Nombre(), err)
		}
		permisos := os.FileMode(0o644)
		if f.Ejecutable() {
			permisos = 0o755
		}
		if err := os.WriteFile(ruta, []byte(interpolado), permisos); err != nil {
			return fmt.Errorf("ejecución: reescribir la plantilla %q del paso %q: %w", f.Ruta(), paso.Nombre(), err)
		}
	}
	return nil
}
