package infraestructura

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jairoprogramador/vex-engine/internal/definicion/dominio"
)

// leer convierte el directorio de un pipeline en una declaración, tal como está escrita (modelo/contextos/
// definicion.md, «Los ficheros del pipeline»). No decide nada sobre lo que lee: lo que no puede leer como YAML,
// o trae claves que el formato no tiene, va a Ilegibles; lo que no reconoce bajo steps/ o variables/, a
// Desconocidos. Solo devuelve error si no puede leer el disco.
func leer(raiz string) (dominio.PipelineDeclarado, error) {
	l := &lectura{raiz: raiz}
	if err := l.version(); err != nil {
		return dominio.PipelineDeclarado{}, err
	}
	if err := l.ambientes(); err != nil {
		return dominio.PipelineDeclarado{}, err
	}
	if err := l.pasos(); err != nil {
		return dominio.PipelineDeclarado{}, err
	}
	if err := l.variables(); err != nil {
		return dominio.PipelineDeclarado{}, err
	}
	return l.d, nil
}

type lectura struct {
	raiz string
	d    dominio.PipelineDeclarado
}

type pipelineV1 struct {
	SchemaVersion *string                    `yaml:"schema_version"`
	Steps         map[string]configuracionV1 `yaml:"steps"`
}

type ambienteV1 struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Value       string `yaml:"value"`
}

type comandoV1 struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description"`
	Cmd         string     `yaml:"cmd"`
	Workdir     string     `yaml:"workdir"`
	Templates   []string   `yaml:"templates"`
	Outputs     []salidaV1 `yaml:"outputs"`
}

type salidaV1 struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Probe       string `yaml:"probe"`
	Scope       string `yaml:"scope"`
}

type configuracionV1 struct {
	Rules  *[]string `yaml:"rules"`
	MaxAge string    `yaml:"max_age"`
	Scope  string    `yaml:"scope"`
}

type variableV1 struct {
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Value       *string `yaml:"value"`
}

// version lee schema_version sin exigir las claves del formato: un config.yaml de otra versión tiene
// que poder decir cuál es, para que la comprobación diga cuál se esperaba. Si es VersionDelFormato, se vuelve
// a leer exigiéndolas, y de ahí sale también la configuración de cada paso, bajo steps (RD-04 §9.20).
func (l *lectura) version() error {
	const fichero = "config.yaml"
	contenido, existe, err := l.fichero(fichero)
	if err != nil || !existe {
		return err
	}
	l.d.Configuracion.Existe = true
	var permisivo pipelineV1
	if err := decodificar(contenido, &permisivo, false); err != nil {
		l.ilegible(fichero, err)
		return nil
	}
	l.d.Configuracion.Datos.Version = permisivo.SchemaVersion
	if permisivo.SchemaVersion == nil || *permisivo.SchemaVersion != dominio.VersionDelFormato {
		return nil
	}
	var estricto pipelineV1
	if err := decodificar(contenido, &estricto, true); err != nil {
		l.ilegible(fichero, err)
		return nil
	}
	if len(estricto.Steps) > 0 {
		l.d.Configuracion.Datos.Pasos = map[string]dominio.ConfiguracionDePasoDeclarada{}
		for nombre, c := range estricto.Steps {
			configuracion := dominio.ConfiguracionDePasoDeclarada{EdadMaxima: c.MaxAge, Ambito: c.Scope}
			if c.Rules != nil {
				configuracion.Reglas = *c.Rules
				configuracion.ReglasEscritas = true
			}
			l.d.Configuracion.Datos.Pasos[nombre] = configuracion
		}
	}
	return nil
}

func (l *lectura) ambientes() error {
	const fichero = "environments.yaml"
	contenido, existe, err := l.fichero(fichero)
	if err != nil || !existe {
		return err
	}
	l.d.Ambientes.Existe = true
	var ambientes []ambienteV1
	if err := decodificar(contenido, &ambientes, true); err != nil {
		l.ilegible(fichero, err)
		return nil
	}
	for _, a := range ambientes {
		l.d.Ambientes.Datos = append(l.d.Ambientes.Datos, dominio.AmbienteDeclarado{
			Nombre: a.Name, Descripcion: a.Description, Valor: a.Value,
		})
	}
	return nil
}

func (l *lectura) pasos() error {
	raiz := filepath.Join(l.raiz, "steps")
	if esDirectorio, err := l.directorio(raiz, "steps"); err != nil || !esDirectorio {
		return err
	}
	entradas, err := os.ReadDir(raiz)
	if err != nil {
		return fmt.Errorf("definicion: leer steps/: %w", err)
	}
	for _, e := range entradas {
		if !e.IsDir() {
			l.d.Desconocidos = append(l.d.Desconocidos, "steps/"+e.Name())
			continue
		}
		paso, err := l.paso(e.Name())
		if err != nil {
			return err
		}
		l.d.Pasos = append(l.d.Pasos, paso)
	}
	return nil
}

func (l *lectura) paso(directorio string) (dominio.PasoDeclarado, error) {
	base := "steps/" + directorio
	paso := dominio.PasoDeclarado{Directorio: directorio}

	contenido, existe, err := l.fichero(base + "/commands.yaml")
	if err != nil {
		return paso, err
	}
	if existe {
		paso.Comandos.Existe = true
		var comandos []comandoV1
		if err := decodificar(contenido, &comandos, true); err != nil {
			l.ilegible(base+"/commands.yaml", err)
		}
		for _, c := range comandos {
			comando := dominio.ComandoDeclarado{
				Nombre: c.Name, Descripcion: c.Description, Linea: c.Cmd, Directorio: c.Workdir,
				Plantillas: c.Templates,
			}
			for _, s := range c.Outputs {
				comando.Variables = append(comando.Variables, dominio.VariableDeComandoDeclarada{
					Nombre: s.Name, Descripcion: s.Description, Expresion: s.Probe, Ambito: s.Scope,
				})
			}
			paso.Comandos.Datos = append(paso.Comandos.Datos, comando)
		}
	}

	material, err := l.material(base)
	paso.Material = material
	return paso, err
}

// material es todo lo del directorio del paso salvo su commands.yaml de la raíz: un terraform/commands.yaml
// es un fichero más. La configuración del paso ya no vive en su directorio: está en config.yaml.
func (l *lectura) material(base string) ([]dominio.FicheroDeclarado, error) {
	raiz := filepath.Join(l.raiz, filepath.FromSlash(base))
	var material []dominio.FicheroDeclarado
	err := filepath.WalkDir(raiz, func(camino string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relativa, err := filepath.Rel(raiz, camino)
		if err != nil {
			return err
		}
		ruta := filepath.ToSlash(relativa)
		switch {
		case e.IsDir():
			return nil
		case ruta == "commands.yaml":
			return nil
		case e.Type()&fs.ModeSymlink != 0:
			destino, err := os.Readlink(camino)
			if err != nil {
				return err
			}
			material = append(material, dominio.FicheroDeclarado{Ruta: ruta, Enlace: filepath.ToSlash(destino)})
		case e.Type().IsRegular():
			info, err := e.Info()
			if err != nil {
				return err
			}
			contenido, err := os.ReadFile(camino)
			if err != nil {
				return err
			}
			material = append(material, dominio.FicheroDeclarado{
				Ruta: ruta, Contenido: string(contenido), Ejecutable: info.Mode()&0o111 != 0,
			})
		default:
			l.d.Desconocidos = append(l.d.Desconocidos, base+"/"+ruta)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("definicion: leer el material de %s: %w", base, err)
	}
	return material, nil
}

// variables lee los ficheros de variables/: los de la raíz declaran el ámbito compartido, y los de
// variables/<ambiente>/, el de ese ambiente. El nombre del fichero solo organiza, así que cualquiera vale.
// Nada más cabe en variables/.
func (l *lectura) variables() error {
	raiz := filepath.Join(l.raiz, "variables")
	if esDirectorio, err := l.directorio(raiz, "variables"); err != nil || !esDirectorio {
		return err
	}
	err := filepath.WalkDir(raiz, func(camino string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relativa, err := filepath.Rel(raiz, camino)
		if err != nil {
			return err
		}
		ruta := filepath.ToSlash(relativa)
		fichero := "variables/" + ruta
		partes := strings.Split(ruta, "/")
		switch {
		case ruta == ".":
			return nil
		case e.IsDir() && len(partes) == 1:
			return nil
		case e.IsDir():
			l.d.Desconocidos = append(l.d.Desconocidos, fichero+"/")
			return filepath.SkipDir
		case !e.Type().IsRegular() || path.Ext(ruta) != ".yaml" || len(partes) > 2:
			l.d.Desconocidos = append(l.d.Desconocidos, fichero)
			return nil
		}
		escritas := dominio.VariablesDePipelineDeclarada{Fichero: fichero}
		if len(partes) == 2 {
			escritas.Ambito = partes[0]
		}
		contenido, err := os.ReadFile(camino)
		if err != nil {
			return err
		}
		var variables []variableV1
		if err := decodificar(contenido, &variables, true); err != nil {
			l.ilegible(fichero, err)
		}
		for _, v := range variables {
			escritas.Variables = append(escritas.Variables, dominio.VariableDePipelineDeclarada{
				Nombre: v.Name, Descripcion: v.Description, Valor: v.Value,
			})
		}
		l.d.Variables = append(l.d.Variables, escritas)
		return nil
	})
	if err != nil {
		return fmt.Errorf("definicion: leer variables/: %w", err)
	}
	return nil
}

// fichero lee un fichero del formato. Si está pero no es un fichero, como un directorio o un enlace, es
// ilegible: un enlace podría leer algo de fuera del pipeline.
func (l *lectura) fichero(ruta string) (contenido []byte, existe bool, err error) {
	camino := filepath.Join(l.raiz, filepath.FromSlash(ruta))
	info, err := os.Lstat(camino)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("definicion: leer %s: %w", ruta, err)
	case !info.Mode().IsRegular():
		l.ilegible(ruta, errors.New("no es un fichero"))
		return nil, true, nil
	}
	contenido, err = os.ReadFile(camino)
	if err != nil {
		return nil, false, fmt.Errorf("definicion: leer %s: %w", ruta, err)
	}
	return contenido, true, nil
}

// directorio dice si steps/ o variables/ son un directorio que se puede recorrer. Si no están, no hay nada que
// leer; si están y no son un directorio, como un fichero o un enlace, no son parte del formato.
func (l *lectura) directorio(camino, ruta string) (bool, error) {
	info, err := os.Lstat(camino)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("definicion: leer %s: %w", ruta, err)
	case !info.IsDir():
		l.d.Desconocidos = append(l.d.Desconocidos, ruta)
		return false, nil
	}
	return true, nil
}

func (l *lectura) ilegible(fichero string, err error) {
	l.d.Ilegibles = append(l.d.Ilegibles, dominio.FicheroIlegibleDeclarado{Fichero: fichero, Motivo: err.Error()})
}

// decodificar lee un YAML. Vacío no es un error: no declara nada. Estricto rechaza las claves que el formato no
// tiene, como el rules: - state_changed de la versión 2.
func decodificar(contenido []byte, destino any, estricto bool) error {
	decodificador := yaml.NewDecoder(bytes.NewReader(contenido))
	decodificador.KnownFields(estricto)
	err := decodificador.Decode(destino)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("no se puede leer como el formato pide: %w", err)
	}
	return nil
}
