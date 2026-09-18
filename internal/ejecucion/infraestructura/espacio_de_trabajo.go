package infraestructura

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jairoprogramador/vex-engine/internal/ejecucion/dominio"
)

// EspacioDeTrabajo es el espacio de trabajo de verdad, en disco: bajo raiz/<ambiente>/, la parte del motor vive
// en motor/ (se rehace entera al empezar cada intento, DEC-06.19) y la de la tecnología en tecnologia/, que
// este puerto nunca lee ni escribe — la crea, si hace falta, lo que corren los comandos del pipeline.
type EspacioDeTrabajo struct {
	raiz string
}

var _ dominio.EspacioDeTrabajo = (*EspacioDeTrabajo)(nil)

func NuevoEspacioDeTrabajo(raiz string) *EspacioDeTrabajo {
	return &EspacioDeTrabajo{raiz: raiz}
}

func (e *EspacioDeTrabajo) DirectorioDelAmbiente(ambiente string) string {
	return filepath.Join(e.raiz, ambiente, "motor")
}

func (e *EspacioDeTrabajo) DirectorioDelPaso(ambiente, paso string) string {
	return filepath.Join(e.DirectorioDelAmbiente(ambiente), paso)
}

func (e *EspacioDeTrabajo) RehacerParteDelMotor(_ context.Context, ambiente string, pasos []dominio.PasoDeEjecucion) error {
	motor := e.DirectorioDelAmbiente(ambiente)
	if err := os.RemoveAll(motor); err != nil {
		return fmt.Errorf("ejecución: el ambiente %q: %w: %w", ambiente, dominio.ErrNoDisponible, err)
	}
	for _, paso := range pasos {
		directorioDelPaso := e.DirectorioDelPaso(ambiente, paso.Nombre())
		if err := os.MkdirAll(directorioDelPaso, 0o755); err != nil {
			return fmt.Errorf("ejecución: el ambiente %q: %w: %w", ambiente, dominio.ErrNoDisponible, err)
		}
		for _, f := range paso.Material {
			if err := escribirFichero(directorioDelPaso, f); err != nil {
				return fmt.Errorf("ejecución: el ambiente %q: %w: %w", ambiente, dominio.ErrNoDisponible, err)
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
	_ context.Context, ambiente string, paso dominio.PasoDeEjecucion, interpolar dominio.Interpolador,
) error {
	directorioDelPaso := e.DirectorioDelPaso(ambiente, paso.Nombre())
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
